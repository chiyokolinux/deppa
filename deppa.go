/**
 * This file is part of Deppa.

 * Deppa is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * Deppa is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with Deppa.  If not, see <https://www.gnu.org/licenses/>.
**/

package main

import (
	"fmt"
	"os"
	"net"
	"flag"
	"bufio"
	"strconv"
	"strings"
	"io/ioutil"
	"io"
)

type DeppaSettings struct {
	hostname string
	port int
	portString string
	dir string
}

func existsAndIsDir(path string) (bool, bool, error) {
	info, err := os.Stat(path)
	if err == nil {
		if info.IsDir() {
			return true, true, nil
		} else {
			return true, false, nil
		}
	}
	if os.IsNotExist(err) {
		return false, false, nil
	}
	return false, false, err
}

func ErrorResponse(message string) string {
	/* return an error response */
	return "3" + message + "\tfake\t(NULL)\t0\r\n.\r\n"
}

func handleConnection(conn net.Conn, opts DeppaSettings) {
	/* handle incoming requests, parse magic string and call handler */
	defer conn.Close()

	buf := bufio.NewReader(conn)
	req, toobig, err := buf.ReadLine()
	if err != nil {
		fmt.Fprint(conn, ErrorResponse("Invalid request: cannot read magic string"))
		return
	}
	if toobig {
		fmt.Fprint(conn, ErrorResponse("Invalid request: magic string too big"))
		return
	}

	fmt.Printf("%v: %s\n", conn.RemoteAddr(), req)

	handleBasicRequest(string(req), conn, opts)
}

func handleBasicRequest(request string, conn net.Conn, opts DeppaSettings) {
	/* find request type */
	if request[0] == '/' {
		request = request[1:]
	}
	if strings.Contains(request, "../") || strings.Contains(request, "/..") {
		fmt.Fprint(conn, ErrorResponse("Invalid request: \"../\" and \"/..\" are not allowed in magic string"))
	}

	if request[len(request) - 1] == '/' || request == "" {
		handleDirectoryListingRequest(request, conn, opts)
	}
}

func handleDirectoryListingRequest(request string, conn net.Conn, opts DeppaSettings) {
	files, err := ioutil.ReadDir(opts.dir + "/" + request)
	if err != nil {
		fmt.Println(opts.dir + "/" + request)
		fmt.Fprint(conn, ErrorResponse("Invalid request: cannot read target dir"))
	}

	reverse := false
	footer := false
	header := false
	use_index := false
	var resplines []string
	var index_fname string

	for _, file := range files {
		if file.Name()[0] == '.' {
			if file.Name() == ".reverse" {
				reverse = true
			} else if file.Name() == ".header" {
				header = true
			} else if file.Name() == ".footer" {
				footer = true
			} else if strings.HasPrefix(file.Name(), "index") && !strings.HasSuffix(file.Name(), ".html") {
				use_index = true
				index_fname = file.Name()
				break
			}
			continue
		}

		var respline string
		if file.IsDir() || strings.HasSuffix(file.Name(), ".md") || strings.HasSuffix(file.Name(), ".gm") {
			respline = "1" + file.Name() + "\t" + request + "/" + file.Name() + "\t" + opts.hostname + "\t" + opts.portString + "\r\n"
		} else if strings.HasSuffix(file.Name(), ".gobj") || strings.HasSuffix(file.Name(), ".txt") {
			respline = "0" + file.Name() + "\t" + request + "/" + file.Name() + "\t" + opts.hostname + "\t" + opts.portString + "\r\n"
		} else {
			respline = "9" + file.Name() + "\t" + request + "/" + file.Name() + "\t" + opts.hostname + "\t" + opts.portString + "\r\n"
		}
		resplines = append(resplines, respline)
	}

	if header {
		SendFile(opts.dir + "/" + request + "/.header", conn)
	}
	if use_index {
		handleFileDisplayRequest(request + "/" + index_fname, conn, opts, false)
	} else {
		if reverse {
			for i := len(resplines) - 1; i >= 0; i-- {
				fmt.Fprint(conn, resplines[i])
			}
		} else {
			for _, entry := range resplines {
				fmt.Fprint(conn, entry)
			}
		}
	}
	if footer {
		SendFile(opts.dir + "/" + request + "/.header", conn)
	}
	fmt.Fprint(conn, ".\r\n")
}

func handleFileDisplayRequest(request string, conn net.Conn, opts DeppaSettings, standalone bool) {
	if strings.HasSuffix(request, ".md") {

	} else if strings.HasSuffix(request, ".gm") {

	} else if strings.HasSuffix(request, ".gobj") {

	} else if strings.HasSuffix(request, ".txt") {
		SendFile(opts.dir + "/" + request, conn)
		fmt.Fprint(conn, ".\r\n")
	} else {
		SendFile(opts.dir + "/" + request, conn)
		return
	}
	if standalone {
		fmt.Fprint(conn, ".\r\n")
	}
}

func SendHeaderIfStandaloneAndExists(request string, conn net.Conn, opts DeppaSettings, standalone bool) {
	if standalone {
		pathParts := strings.Split(request, "/")
		var path string
		if len(pathParts) == 1 {
			path = ""
		} else {
			path = strings.Join(pathParts[:len(pathParts) - 1], "/")
		}

		exist, isdir, err := existsAndIsDir(opts.dir + "/" + path + "/.header")
		if exist && !isdir && err == nil {
			SendFile(opts.dir + "/" + path + "/.header", conn)
		}
	}
}

func SendFooterIfStandaloneAndExists(request string, conn net.Conn, opts DeppaSettings, standalone bool) {
	if standalone {
		pathParts := strings.Split(request, "/")
		var path string
		if len(pathParts) == 1 {
			path = ""
		} else {
			path = strings.Join(pathParts[:len(pathParts) - 1], "/")
		}

		exist, isdir, err := existsAndIsDir(opts.dir + "/" + path + "/.footer")
		if exist && !isdir && err == nil {
			SendFile(opts.dir + "/" + path + "/.footer", conn)
		}
	}
}

func SendFile(path string, conn net.Conn) {
	file, err := os.Open(path)
	if err != nil {
		fmt.Fprint(conn, ErrorResponse("Not found"))
		return
	}
	defer file.Close()
	_, err = io.Copy(conn, file)
	if err != nil {
		return
	}
}

func RunServer(opts DeppaSettings) {
	/* init socket */
	ln, err := net.Listen("tcp", opts.hostname + ":" + opts.portString)
	if err != nil {
		fmt.Printf("error: could not listen on socket. exiting. (%v)\n", err)
		return
	}

	fmt.Println("listening on " + opts.hostname + ":" + opts.portString)

	/* listen forever */
	for {
		conn, err := ln.Accept()
		if err != nil {
			fmt.Printf("error: could not accept connection (%v)\n", err)
		}
		go handleConnection(conn, opts)
	}
}

func main() {
	/* determine hostname */
	default_hostname, err := os.Hostname()
	if err != nil {
		fmt.Printf("error: could not fetch default hostname, defaulting to 0.0.0.0 (%v)\n", err)
		default_hostname = "0.0.0.0"
	}

	/* parse flags */
	hostname := flag.String("h", default_hostname, "hostname to listen on")
	port := flag.Int("p", 70, "port to listen on")
	dir := flag.String("d", ".", "directory to serve files from")
	flag.Parse()

	opts := DeppaSettings { *hostname, *port, strconv.Itoa(*port), *dir }

	RunServer(opts)
}

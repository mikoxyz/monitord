// SPDX-License-Identifier: MIT
/*
 * monitord - a stupid daemon that monitors the status of an arbritary amount of systems using ping
 */

package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"time"

	"github.com/prometheus-community/pro-bing"
)

type Flags struct {
	host_list_path *string
	runtime_dir    *string
	timeout        *time.Duration
}

func check_if_path_exists(path string, error_object string) {
	/* this is actually unused, but whatevs */
	if len(error_object) <= 0 {
		error_object = "path"
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		fmt.Println(error_object, "does not exist")
		os.Exit(1)
	} else {
		err_check(err)
	}
}

func create_host_file(host string) *os.File {
	f, err := os.Create(host)

	err_check(err)
	return f
}

/* this is where errors that we don't handle go */
func err_check(err error) {
	if err != nil {
		panic(err)
	}
}

func get_pinger(host string) (*probing.Pinger, bool) {
	pinger, err := probing.NewPinger(host)
	dns_err := false

	/* i'm not sure this is the right way to go about this, but it does the trick */
	var dns_error *net.DNSError
	if errors.As(err, &dns_error) {
		dns_err = true
	} else {
		err_check(err)
	}

	/*
	 * i'm not sure pro-bing supports unprivileged icmp on the BSDs, so let's just set pinger
	 * to be privileged
	 */
	pinger.SetPrivileged(true)
	return pinger, dns_err
}

func monitor_handler(host string, timeout time.Duration) {
	pinger, dns_err := get_pinger(host)

	if dns_err == false {
		pinger.Count = 1
		pinger.Timeout = timeout

		err := pinger.Run()

		err_check(err)
	}

	f := create_host_file(host)

	defer f.Close()

	/*
	 * TODO: check if this behaves as expected even if we receive something like "packet
	 * filtered"
	 */
	if dns_err == true {
		write_status(f, "hostname couldn't be resolved\n")
	} else if pinger.PacketsRecv == 0 {
		write_status(f, "down\n")
	} else {
		write_status(f, "up\n")
	}
}

func monitor(host_list []string, timeout time.Duration) {
	for {
		for i := 0; i < len(host_list); i++ {
			go monitor_handler(host_list[i], timeout)
		}

		timer := time.NewTimer(1 * time.Minute)
		<-timer.C
	}
}

func open_host_list(host_list_path string) *os.File {
	check_if_path_exists(host_list_path, "host list")

	host_list_file, err := os.Open(host_list_path)

	err_check(err)
	return host_list_file
}

func parse_flags() Flags {
	flags := Flags{
		host_list_path: flag.String("l", "/etc/monitord/host_list", "path to host list"),
		runtime_dir:    flag.String("r", "/run/monitord", "path to the runtime directory"),
		timeout:        flag.Duration("t", time.Second*10, "timeout for ping"),
	}

	flag.Parse()
	return flags
}

func parse_host_list(host_list_file *os.File) []string {
	defer host_list_file.Close()

	s := bufio.NewScanner(host_list_file)
	var host_list []string
	for s.Scan() {
		host_list = append(host_list, s.Text())
	}

	return host_list
}

func write_status(f *os.File, status string) {
	_, err := f.WriteString(status)

	err_check(err)
}

func main() {
	flags := parse_flags()

	check_if_path_exists(*flags.runtime_dir, "runtime directory")
	os.Chdir(*flags.runtime_dir)
	monitor(parse_host_list(open_host_list(*flags.host_list_path)), *flags.timeout)
}

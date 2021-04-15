package main

import (
	"bufio"
	"context"
	"encoding/binary"
	"fmt"
	"github.com/posener/complete"
	"io/fs"
	"net"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func withArgs(p complete.Predictor) complete.Command {
	return complete.Command{
		Args: p,
	}
}

func computeArgs(f func(a complete.Args) []string) complete.Command {
	return complete.Command{
		Args: complete.PredictFunc(f),
		Flags: map[string]complete.Predictor{
			"-s": complete.PredictFunc(getDevices),
		},
	}
}

func main() {
	//fmt.Printf("%q", getDevices(complete.Args{}))
	//os.Exit(0)
	//fmt.Printf("%+v\n", getHost(complete.Args{}))
	//println("Done")
	//os.Exit(0)
	c := complete.New("adb", complete.Command{
		Sub: complete.Commands{
			"disconnect": computeArgs(getDevices),
			"uninstall":  computeArgs(getPackages),
			"install":    computeArgs(getApks),
			"shell": {
				Sub: map[string]complete.Command{
					"pm": {
						Sub: map[string]complete.Command{
							"clear": computeArgs(getPackages),
						},
					},
				},
				Args: complete.PredictSet("am broadcast -a", "pm clear"),
			},
			"connect": computeArgs(getHost),
			"tcpip":   withArgs(complete.PredictSet("5555")),
		},
		//Args: anyOf("uninstall", "tcpip", "install", "devices", "shell"),
		GlobalFlags: map[string]complete.Predictor{
			"-s": complete.PredictFunc(getDevices),
		},
	})
	c.Complete()
}

func getDevices(complete.Args) (out []string) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "adb", "devices")
	b, err := cmd.StdoutPipe()
	if err != nil {
		panic(err)
	}
	if err := cmd.Start(); err != nil {
		panic(err)
	}
	scanner := bufio.NewScanner(b)
	scanner.Scan()
	for scanner.Scan() {
		elems := strings.Split(scanner.Text(), "\t")[0]
		if elems != "" {
			out = append(out, elems)
		}
	}
	return
}

func getApks(complete.Args) (out []string) {
	err := filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
		if !d.IsDir() && strings.HasSuffix(d.Name(), ".apk") {
			out = append(out, path)
		}
		return nil
	})
	if err != nil {
		panic(err)
	}
	return
}

func getPackages(arguments complete.Args) (out []string) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	args := []string{"shell", "pm", "list", "packages"}
	host := getDeviceFromArgs(arguments)
	if host != "" {
		args = append([]string{"-s", host}, args...)
	}
	cmd := exec.CommandContext(ctx, "adb", args...)
	b, err := cmd.StdoutPipe()
	if err != nil {
		panic(err)
	}
	if err = cmd.Start(); err != nil {
		panic(err)
	}
	scanner := bufio.NewScanner(b)
	for scanner.Scan() {
		text := scanner.Text()
		if strings.HasPrefix(text, "package:") {
			out = append(out, strings.TrimPrefix(text, "package:"))
		}
	}
	return
}

func getHost(complete.Args) (out []string) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	ch := make(chan string)
	const SIZE = 200
	queue := make(chan bool, SIZE)
	for _, iterator := range addrIterators() {
		complete.Log("AddrIt: %s\n", iterator)
		if iterator.Start.String()[:4] == "127." {
			continue
		}
		start := binary.BigEndian.Uint32(iterator.Start)
		end := binary.BigEndian.Uint32(iterator.End)
		for i := start; i < end; i++ {
			ip := make(net.IP, 4)
			binary.BigEndian.PutUint32(ip, i)
			go doActualPortScan(ctx, queue, ip, ch)
		}
	}
	for {
		select {
		case <-ctx.Done():
			return out
		case s := <-ch:
			out = append(out, s)
		}
	}
}

func doActualPortScan(ctx context.Context, queue chan bool, ip net.IP, ch chan<- string) {
	ipAddress := fmt.Sprintf("%s:5555", ip)
	queue <- true
	defer func() {
		<-queue
		complete.Log("Queue drained")
	}()
	complete.Log("Dialing %s", ipAddress)
	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", ipAddress)
	if err == nil {
		_ = conn.Close()
		ch <- ipAddress
	} else {
		complete.Log("dial failed %s", err)
	}
}

func addrIterators() (out []AddrIterator) {
	ifaces, err := net.Interfaces()
	if err != nil {
		complete.Log("localAddresses: %+v\n", err.Error())
		return nil
	}
	for _, i := range ifaces {
		addrs, err := i.Addrs()
		if err != nil {
			continue
		}
		for _, a := range addrs {
			switch v := a.(type) {
			case *net.IPNet:
				meh := createAddrIterator(v.IP)
				if meh != nil {
					out = append(out, *meh)
				}
			}
		}
	}
	return out
}

func createAddrIterator(ip net.IP) *AddrIterator {
	start := ip.To4()
	if start == nil {
		return nil
	}
	end := make([]byte, len(start))
	copy(end, start)

	start[3] = 0
	end[3] = 255
	return &AddrIterator{
		Start: start,
		End:   end,
	}
}

type AddrIterator struct {
	Start net.IP
	End   net.IP
}

func (a AddrIterator) String() string {
	return fmt.Sprintf("start: [%+v], end: [%+v]", a.Start, a.End)
}

func getDeviceFromArgs(a complete.Args) string {
	if strs, ok := a.GlobalArguments["-s"]; ok && len(strs) > 0 {
		return strs[0]
	}
	return ""
}

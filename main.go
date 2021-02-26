package main

import (
	"bufio"
	"context"
	"fmt"
	"github.com/posener/complete"
	"io/fs"
	"net"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

func predict(f func(a complete.Args) []string) complete.PredictFunc {
	return f
}

func main() {
	//fmt.Printf("%q", getDevices(complete.Args{}))
	//os.Exit(0)
	//fmt.Printf("%+v\n", getHost(complete.Args{}))
	//println("Done")
	//os.Exit(0)
	c := complete.New("adb", complete.Command{
		Sub: complete.Commands{
			"disconnect": complete.Command{
				Args: predict(getDevices),
			},
			"uninstall": complete.Command{
				Args: predict(getPackages),
			},
			"install": complete.Command{
				Args: predict(install),
			},
			"shell": complete.Command{
				Sub: map[string]complete.Command{
					"pm": {
						Sub: map[string]complete.Command{
							"clear": {
								Args: predict(getPackages),
							},
						},
					},
				},
				Args: predict(shellExpansions),
			},
			"connect": complete.Command{
				Args: predict(getHost),
			},
			"tcpip": complete.Command{
				Args: predict(func(a complete.Args) []string {
					return []string{"5555"}
				}),
			},
		},
		Args: predict(func(a complete.Args) []string {
			return []string{"uninstall", "tcpip", "install", "devices", "shell"}
		}),
		Flags: map[string]complete.Predictor{
			"-s": predict(getDevices),
		},
	})
	c.Complete()
}

func shellExpansions(complete.Args) []string {
	return []string{"am broadcast -a", "pm clear"}
}

func getDevices(complete.Args) (out []string) {
	cmd := exec.Command("adb", "devices")
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

func install(complete.Args) (out []string) {
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

func getPackages(complete.Args) (out []string) {
	cmd := exec.Command("adb", "shell", "pm", "list", "packages")
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
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	var wg sync.WaitGroup
	wg.Add(255 - 2)
	ch := make(chan string)
	for i := 2; i < 255; i++ {
		go func(i int) {
			doActualPortScan(ctx, i, ch)
			wg.Done()
		}(i)
	}
	go func() {
		wg.Wait()
		close(ch)
	}()
	for s := range ch {
		out = append(out, s)
	}
	return out
}

func doActualPortScan(ctx context.Context, i int, ch chan<- string) {
	result := make(chan string, 1)
	go func() {
		ipAddress := fmt.Sprintf("192.168.1.%d:5555", i)
		complete.Log("Dialing %s", ipAddress)
		conn, err := net.Dial("tcp", ipAddress)
		if err != nil {
			complete.Log("%d sket sig", i)
		} else {
			complete.Log("%d gick bra", i)
			err = conn.Close()
			result <- ipAddress
			if err != nil {
				complete.Log("close failed %s\n", err)
			}
		}
	}()
	select {
	case <-ctx.Done():
	case str := <-result:
		ch <- str
	}
}

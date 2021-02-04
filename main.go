package main

import (
	"bufio"
	"github.com/posener/complete"
	"io/fs"
	"os/exec"
	"path/filepath"
	"strings"
)

func predict(f func(a complete.Args) []string) complete.PredictFunc {
	return f
}

func main() {
	c := complete.New("adb", complete.Command{
		Sub: complete.Commands{
			"uninstall": complete.Command{
				Args: predict(getPackages),
			},
			"install": complete.Command{
				Args: predict(install),
			},
			"shell": complete.Command{
				Args: predict(shellExpansions),
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
	return []string{"am broadcast -a"}
}

func getDevices(complete.Args) (out []string) {
	b, err := exec.Command("adb", "devices").StdoutPipe()
	if err != nil {
		panic(err)
	}
	scanner := bufio.NewScanner(b)
	scanner.Scan()
	for scanner.Scan() {
		out = append(out, strings.Split(scanner.Text(), "\t")[0])
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
	b, err := exec.Command("adb", "shell", "pm", "list", "packages").StdoutPipe()
	if err != nil {
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

package main

import (
	"bufio"
	"context"
	"github.com/posener/complete"
	"os/exec"
	"strings"
	"time"
)

func main() {
	c := complete.New("adb", complete.Command{
		Sub: complete.Commands{
			"uninstall":  complete.Command{
				Args: complete.PredictFunc(getPackages),
			},
			"connect": complete.Command{
				// hardcode for show
				Args: complete.PredictSet("192.168.1.64"),
			},
		},
		//Args: anyOf("uninstall", "tcpip", "install", "devices", "shell"),
		GlobalFlags: map[string]complete.Predictor{
			"-s": complete.PredictSet("192.168.1.64"),
		},
	})
	c.Complete()
}

func getPackages(arguments complete.Args) (out []string) {
	// I WANT my -s argument
	var host string
	// but from where? arguments is empty

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	args := []string{"shell", "pm", "list", "packages"}

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

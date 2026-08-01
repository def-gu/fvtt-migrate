package main

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/def-gu/fvtt-migrate/internal/api"
	"github.com/def-gu/fvtt-migrate/internal/foundry"
)

const launchAddress = "127.0.0.1:7788"

// Double-clicking the program lands here. It has to succeed or explain itself
// without anyone typing anything, so the messages are addressed to the person
// whose campaign this is rather than to an operator.
func runLaunch() error {
	inst, err := foundry.Discover()
	if err != nil {
		fmt.Println("Не удалось найти установку Foundry в обычных местах.")
		fmt.Println()
		fmt.Println("Искали здесь:")
		for _, p := range foundry.Candidates() {
			fmt.Println("  ", p)
		}
		fmt.Println()
		fmt.Println("Если ваша установка лежит в другом месте, запустите с указанием пути:")
		fmt.Printf("  %s panel --root <путь до папки с Data, Config и Logs>\n", self())
		wait()
		return nil
	}

	if alreadyRunning() {
		fmt.Println("Панель уже запущена. Открываю её в браузере.")
		openBrowser("http://" + launchAddress + "/")
		wait()
		return nil
	}

	fmt.Printf("Установка       %s\n", inst.Root)
	fmt.Printf("Панель          http://%s/\n\n", launchAddress)
	fmt.Println("Браузер откроется сам. Это окно можно свернуть.")
	fmt.Println("Закрытие этого окна останавливает панель.")

	go func() {
		time.Sleep(700 * time.Millisecond)
		openBrowser("http://" + launchAddress + "/")
	}()

	if err := servePanel(inst.Root, "", launchAddress); err != nil {
		fmt.Println()
		fmt.Println("Панель остановилась:", err)
		wait()
	}
	return nil
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	cmd.Start()
}

// A window opened by a double click closes the moment the program ends, taking
// the explanation with it.
func wait() {
	fmt.Println()
	fmt.Println("Нажмите Enter, чтобы закрыть это окно.")
	fmt.Fscanln(os.Stdin)
}

func self() string {
	path, err := os.Executable()
	if err != nil {
		return "fvtt-migrate"
	}
	name := path[strings.LastIndexAny(path, `/\`)+1:]
	if name == "" {
		return "fvtt-migrate"
	}
	return name
}

func alreadyRunning() bool {
	client := http.Client{Timeout: time.Second}
	resp, err := client.Get("http://" + launchAddress + api.PathPing)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

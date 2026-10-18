package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"runtime"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"
	"tavu/internal/views"
)

func main() {
	mygo.App.SetName("Tavu")
	mygo.App.SetLanguages("en")
	if len(os.Args) > 1 && os.Args[1] == "--diagnostics" {
		diagnostics()
		return
	}
	if !mygo.App.RequestSingleInstanceLock() {
		return
	}
	mygo.App.WhenReady(func() {
		mygo.App.SetLocale("en")
		mygo.Theme.SetSource(mygo.ThemeDark)
		data, err := mygo.App.Path(mygo.PathUserData)
		if err != nil {
			log.Print(err)
		}
		a := views.New(data)
		win := mygo.NewWindow(mygo.WindowOptions{Title: "Tavu", Width: 1380, Height: 920, MinWidth: 760, MinHeight: 560, StateKey: "main", BackgroundColor: "#0d0f0e", TitleBarStyle: mygo.TitleBarHidden, TrafficLightPosition: &mygo.Point{X: 14, Y: 15}, TitleBarHeight: 44, Content: ui.View(a.View)})
		a.Attach(win)
		mygo.App.OnSecondInstance(func(_ []string, _ string) { win.Show(); win.Focus() })
		mygo.App.SetMenu(a.Menu())
		path := ""
		if len(os.Args) > 1 {
			path = os.Args[1]
		}
		a.Start(path)
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}

func diagnostics() {
	executable, _ := os.Executable()
	library, err := terminal.LibraryPath()
	if err == nil {
		err = terminal.Load()
	}
	result := map[string]any{"product": "Tavu", "version": "0.1.0", "mygo": "v0.5.5", "go": runtime.Version(), "platform": runtime.GOOS + "/" + runtime.GOARCH, "executable": executable, "terminalLibrary": library, "libraryLoaded": err == nil}
	if err != nil {
		result["error"] = err.Error()
	}
	data, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(data))
	if err != nil {
		os.Exit(1)
	}
}

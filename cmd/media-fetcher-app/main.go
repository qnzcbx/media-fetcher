package main

import (
	"log"

	"github.com/egoist/mygo"
	"media-fetcher/internal/desktop"
)

func main() {
	svc, err := desktop.NewDesktopService()
	if err != nil {
		log.Fatalf("Failed to start desktop service: %v", err)
	}

	mygo.Bind(svc)

	mygo.App.WhenReady(func() {
		win := mygo.NewWindow(mygo.WindowOptions{
			Title:           "MFetch",
			Width:           960,
			Height:          680,
			MinWidth:        800,
			MinHeight:       600,
			BackgroundColor: "light-dark(#f8f9fa, #202427)",
			StateKey:        "main",
			URL:             "/",
		})
		svc.SetWindow(win)
	})

	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}

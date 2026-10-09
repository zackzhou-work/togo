package main

import (
	"log"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/zackzhou-work/togo/internal/store"
)

func main() {
	db, err := store.Open(dbPath())
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	mygo.App.WhenReady(func() {
		var win *mygo.Window
		// Timers fire on their own goroutine; the state belongs to the UI thread.
		after := func(d time.Duration, fn func()) {
			time.AfterFunc(d, func() { win.Update(fn) })
		}
		p := loadPrefs()
		a := newApp(db, p.Pinned, after)
		a.setPinned = func(on bool) {
			win.SetAlwaysOnTop(on)
			if err := savePrefs(prefs{Pinned: on}); err != nil {
				log.Printf("[prefs] %v", err)
			}
		}

		win = mygo.NewWindow(mygo.WindowOptions{
			Title:  "togo",
			Width:  380,
			Height: 520,
			// Narrower and the hover tray covers most of a title; shorter and
			// there is no room for both headings and a row under either.
			MinWidth:             260,
			MinHeight:            220,
			StateKey:             "main",
			TitleBarStyle:        mygo.TitleBarHidden,
			TrafficLightPosition: &mygo.Point{X: 14, Y: 11},
			AlwaysOnTop:          p.Pinned,
			Content:              ui.View(a.view),
		})
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}

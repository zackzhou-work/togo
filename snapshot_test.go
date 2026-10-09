package main

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

// SNAP=<dir> go test -run TestSnapshots writes PNGs of the main states, for
// checking the look by eye.
func TestSnapshots(t *testing.T) {
	out := os.Getenv("SNAP")
	if out == "" {
		t.Skip()
	}
	save := func(h *harness, name string) {
		h.tt.Frame()
		f, _ := os.Create(filepath.Join(out, name+".png"))
		png.Encode(f, h.tt.Image())
		f.Close()
	}
	h := newHarness(t, []string{"写周报", "回复设计评审的评论，顺便把上周遗留的那个很长很长的问题也处理掉"}, []string{"买牛奶", "约牙医"})
	h.tt.SetScale(2)
	h.app.togglePriority(h.app.ongoing[0].ID)
	save(h, "1-rest")
	h.hover("回复设计评审的评论，顺便把上周遗留的那个很长很长的问题也处理掉")
	save(h, "2-hover")
	h.click("删除")
	save(h, "3-armed")
	h.hover("买牛奶")
	h.click("完成")
	save(h, "4-held")
	h.clock.advance(time.Second)
	h.tt.Key(ui.Cmd, ui.KeyN)
	save(h, "5-draft")
	h.tt.Key(0, ui.KeyEscape)
	h.tt.Key(ui.Cmd|ui.Shift, ui.KeyH)
	save(h, "6-history")
	h.tt.Key(0, ui.KeyEscape)
	h.tt.Frame()
	x, y := h.center("约牙医")
	_, ty := h.center("写周报")
	h.tt.Press(x, y)
	for yy := y; yy >= ty; yy -= 6 {
		h.tt.Move(x, yy)
		h.tt.Frame()
	}
	save(h, "7-drag")
}

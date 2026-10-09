package main

import (
	"strconv"

	"github.com/egoist/mygo/ui"

	"github.com/zackzhou-work/togo/internal/store"
)

const (
	// The top strip stays clear for the traffic lights. It carries no surface
	// and no rule of its own, and it is the window's drag handle.
	chromeHeight = 34
	gutter       = 12
	rowRadius    = 6
	rowHeight    = 36
	// The tray's left inset doubles as the length of its fade: wide enough that
	// a couple of characters dissolve, no wider, so the first button still lands
	// on solid colour.
	trayFadeWidth = 24
	sectionHeight = 28
	// Where the heading's label starts, which is also where the rows' titles
	// start: heading and rows read as one left edge.
	labelIndent = 16
)

// dragged travels with a row being dragged.
type dragged struct{ id string }

func (a *app) view(c *ui.Context) {
	theme := *c.Theme()
	theme.Accent, theme.Selection, theme.Scrollbar = colorCaret, colorSelection, ui.Transparent
	c.SetTheme(&theme)
	c.Root().Background(colorGround).TextColor(colorInk)
	a.hoveredID = ""
	// Drops and presses outside a field land in the middle of building the
	// lists they change, so they wait until the view is done.
	var later []func()
	defer func() {
		for _, fn := range later {
			fn()
		}
		if len(later) > 0 {
			c.Invalidate()
		}
	}()

	c.OnShortcut(ui.Cmd, ui.KeyN, a.startCreating)
	c.OnShortcut(ui.Cmd|ui.Shift, ui.KeyP, a.togglePin)
	c.OnShortcut(ui.Cmd|ui.Shift, ui.KeyH, a.toggleHistory)
	c.OnShortcut(0, ui.KeyEscape, a.cancel)
	// A focused text field takes ⌘⌫ first, as "delete to the start of the
	// line", so this only fires while nothing is being typed.
	c.OnShortcut(ui.Cmd, ui.KeyBackspace, a.deleteHovered)

	ui.Column(c).Fill().Children(func() {
		historyButton := a.chrome(c)
		// Both sections share one scroller, so the window moves as a single
		// sheet of paper.
		body := ui.Scroll(c).Grow(1).MinHeight(0).TrackScroll(&a.scroll)
		body.Children(func() {
			a.section(c.Key("ongoing"), store.Ongoing, &later)
			ui.Box(c).Height(1).Margin(0, gutter, 6, gutter).Background(colorHairline)
			inbox := a.section(c.Key("inbox"), store.Inbox, &later)
			// ⌘N brings the inbox to the top of the window, where the new
			// field opens, however far down a long ongoing list pushed it.
			if a.revealDraft {
				a.revealDraft = false
				a.scroll.Y += inbox.Bounds().Y - body.Bounds().Y
				c.Invalidate()
			}
		})
		if a.historyOpen {
			a.historyPopover(c, historyButton)
		}
	})
}

// chrome is the top strip: traffic lights on the left (drawn by macOS), the
// app's three actions on the right. It returns the history button, which the
// popover hangs from.
func (a *app) chrome(c *ui.Context) ui.Element {
	var history ui.Element
	ui.Row(c).Height(chromeHeight).Padding(0, gutter).Justify(ui.End).DragWindow().Children(func() {
		ui.Row(c).Gap(2).Children(func() {
			add := actionButton(c, "记一笔", 22, paper, false, func() {
				ui.Icon(c, iconPlus).Size(13, 13).TextColor(colorInkSoft)
			}).OnClick(a.startCreating)
			shortcutTip(c, add, "记一笔", "⌘N")

			history = actionButton(c, "已完成", 22, paper, a.historyOpen, func() {
				ui.Icon(c, iconHistory).Size(12, 12).TextColor(colorInkSoft)
			}).OnClick(a.toggleHistory)
			shortcutTip(c, history, "已完成", "⇧⌘H")

			// Pinning is the one mode that stays on, so it changes shape as well
			// as ink: at 12px a colour shift alone is too quiet.
			pin := actionButton(c, "窗口置顶", 22, paper, false, func() {
				if a.pinned {
					ui.Icon(c, iconPinFilled).Size(12, 12).TextColor(colorInk)
				} else {
					ui.Icon(c, iconPin).Size(12, 12).TextColor(colorInkSoft)
				}
			}).OnClick(a.togglePin)
			shortcutTip(c, pin, "窗口置顶", "⇧⌘P")
		})
	})
	return history
}

func shortcutTip(c *ui.Context, anchor ui.Element, label, keys string) {
	ui.TooltipBase(c, anchor, func(tip ui.Element) {
		tip.Margin(4, 0, 0, 0).Padding(4, 8).Radius(6).Background(colorInk).Children(func() {
			ui.Row(c).Gap(8).Children(func() {
				ui.Text(c, label).FontSize(11.5).TextColor(colorRaised)
				ui.Text(c, keys).FontSize(11.5).TextColor(colorInkFaint)
			})
		})
	})
}

// actionButton is a square hover action. Pressing has to show the instant the
// button goes down, and colour is the channel for it.
func actionButton(c *ui.Context, label string, size float32, s surface, filled bool, icon func()) ui.Element {
	b := ui.Box(c).Size(size, size).Radius(5).Center().Cursor(ui.CursorPointer).Label(label)
	switch {
	case b.Pressed():
		b.Background(s.actionBgActive)
	case b.Hovered():
		b.Background(s.actionBgHover)
	case filled:
		b.Background(s.actionBg)
	}
	return b.Children(icon)
}

// section is one list of tasks: a heading, the rows, then a tail that takes
// drops past the last row and carries the empty copy. Both sections are built
// here so they cannot drift apart.
func (a *app) section(c *ui.Context, column store.Column, later *[]func()) ui.Element {
	label, emptyLine, emptyHint, tasks := "进行中", "还没有开始的事", "从下面拖一件上来", a.ongoing
	if column == store.Inbox {
		label, emptyLine, emptyHint, tasks = "收件箱", "收件箱是空的", "⌘N 记一笔", a.inbox
	}
	showDraft := column == store.Inbox && a.creating

	sec := ui.Column(c).Padding(0, gutter).Gap(1)
	// The fallback for a drop on the heading or in a gap; a row or the tail
	// under the pointer is more specific and takes it first.
	if d, ok := ui.Drop[dragged](sec); ok {
		*later = append(*later, func() { a.reposition(d.id, column, "") })
	}
	sec.Children(func() {
		sectionHeading(c, label, len(tasks))
		if showDraft {
			a.draftRow(c, later)
		}
		for _, t := range tasks {
			a.taskRow(c.Key(t.ID), t, column, later)
		}

		tail := ui.Box(c).MinHeight(12).Center()
		if d, ok := ui.Drop[dragged](tail); ok {
			*later = append(*later, func() { a.reposition(d.id, column, "") })
		}
		_, over := ui.DragOver[dragged](tail)
		tail.Children(func() {
			if len(tasks) == 0 && !showDraft {
				ui.Column(c).PaddingY(18).AlignItems(ui.Center).Gap(3).Children(func() {
					ui.Text(c, emptyLine).FontSize(12.5).TextColor(colorInkSoft)
					ui.Text(c, emptyHint).FontSize(11.5).TextColor(colorInkFaint)
				})
			}
			if over {
				dropIndicator(c)
			}
		})
	})
	return sec
}

func sectionHeading(c *ui.Context, label string, count int) {
	ui.Row(c).Height(sectionHeight).Padding(8, 6, 0, 4).Justify(ui.SpaceBetween).Children(func() {
		ui.Text(c, label).Padding(0, 0, 0, labelIndent).FontSize(11.5).FontWeight(600).TextColor(colorInkSoft)
		ui.Text(c, strconv.Itoa(count)).Padding(1, 5).Radius(8).Background(paper.actionBg).
			FontSize(10.5).FontWeight(600).TextColor(colorInkSoft)
	})
}

// dropIndicator marks where a dragged row will land: a ring on the left end of
// a rule, in the gap above the row it will go before.
func dropIndicator(c *ui.Context) {
	ui.Row(c).Absolute().Top(-3.5).Left(4).Right(6).Height(7).PassThrough().Children(func() {
		ui.Box(c).Size(7, 7).Radius(3.5).Border(1, colorInk)
		ui.Box(c).Grow(1).Height(1.5).Background(colorInk)
	})
}

func (a *app) taskRow(c *ui.Context, t store.Task, column store.Column, later *[]func()) {
	phase := a.completing[t.ID]
	done := phase != 0
	editing := a.editingID == t.ID
	armed := a.confirmDelete == t.ID

	row := ui.Row(c).Padding(0, 6, 0, 4).Radius(rowRadius)
	target := float32(rowHeight)
	if phase == leaving {
		target = 0
	}
	h := row.AnimateWith("collapse", target, completionCollapse, easeOutQuint)
	row.Height(h)
	if h < rowHeight {
		row.Opacity(h / rowHeight).Clip()
	}

	hovered := row.Hovered()
	if hovered {
		// ⌘⌫ acts on whatever the pointer is over.
		a.hoveredID = t.ID
	} else if armed {
		// An armed delete that scrolls out of reach is a trap.
		a.confirmDelete, armed = "", false
	}
	if hovered || editing {
		row.Background(paper.rowHover)
	}
	if editing && row.PressedOutside() {
		*later = append(*later, a.commitEdit)
	}
	// Dropping onto a row puts the dragged one above it, where the indicator
	// promised.
	if d, ok := ui.Drop[dragged](row); ok {
		*later = append(*later, func() { a.reposition(d.id, column, t.ID) })
	}
	_, over := ui.DragOver[dragged](row)

	row.Children(func() {
		// The grip and the title are one drag source: picking a row up anywhere
		// along its title beats aiming at the grip. The tray stays out of it —
		// a few DIPs of travel start a drag, and swallowing a delete that way is
		// worse than the reach it saves. Nothing inside asks for clicks of its
		// own, or it would take the press away from the drag.
		// Keyed, because the drop marker and the tray come and go beside it,
		// and a drag source that loses its identity mid-drag stops dragging.
		lead := ui.Row(c.Key("lead")).Grow(1).MinWidth(0)
		if !editing {
			lead.Drag(dragged{t.ID})
			if lead.DoubleClicked() {
				id := t.ID
				*later = append(*later, func() { a.startEditing(id) })
			}
		}
		grabbing := lead.Pressed()
		lead.Children(func() {
			grip := ui.Box(c).Size(24, 28).Margin(0, -6).Center().Cursor(ui.CursorGrab)
			if grabbing {
				grip.Cursor(ui.CursorGrabbing)
			}
			grip.Children(func() {
				if hovered {
					ui.Icon(c, iconGrip).Size(16, 16).TextColor(colorInkFaint)
				}
			})

			ui.Row(c).Grow(1).MinWidth(0).Gap(7).Padding(0, 0, 0, 4).Children(func() {
				if editing {
					in := ui.TextInputBase(c.Key("edit"), &a.editDraft).Grow(1).MinWidth(0).FontSize(13).AutoFocus().Label("任务标题")
					in.OnSubmit(a.commitEdit)
					return
				}
				title := ui.Text(c, t.Title).SingleLine().Grow(1).MinWidth(0).FontSize(13).Cursor(ui.CursorText)
				if done {
					title.Strikethrough().TextColor(colorInkFaint)
				}
				// After the title, in the same box the tray's flame button will
				// cover, so it does not shift when the tray appears.
				if t.IsPriority {
					ui.Box(c).Size(21, 21).Center().Children(func() {
						ui.Icon(c, iconFlameFilled).Size(11, 11).TextColor(colorPriority)
					})
				}
			})
		})

		if hovered && phase != leaving && !editing {
			a.tray(c, t, done, armed)
		}
		if over {
			dropIndicator(c)
		}
	})
}

// tray holds a row's actions. It sits over the end of the title and fades in
// from transparent, so a long title dissolves under it.
func (a *app) tray(c *ui.Context, t store.Task, done, armed bool) {
	ui.Row(c).Absolute().Top(0).Bottom(0).Right(6).Gap(4).Padding(0, 0, 0, trayFadeWidth).
		LinearGradient(trayFade(paper)).Children(func() {
		del := actionButton(c, "删除", 21, paper, false, func() {
			color := colorInkSoft
			if armed {
				color = colorRaised
			}
			ui.Icon(c, iconTrash).Size(11, 11).TextColor(color)
		}).OnClick(func() { a.requestDelete(t.ID) })
		// Armed. There is no room for words at this size, so the colour is the
		// whole of the warning.
		if armed {
			del.Background(colorPriority)
		}

		// Lit while the row is held struck-through: clicking it again is how
		// the hold is taken back.
		actionButton(c, "完成", 21, paper, done, func() {
			ui.Icon(c, iconCheck).Size(12, 12).TextColor(colorInkSoft)
		}).OnClick(func() { a.completeTask(t.ID) })

		actionButton(c, "优先", 21, paper, false, func() {
			if t.IsPriority {
				ui.Icon(c, iconFlameFilled).Size(11, 11).TextColor(colorPriority)
			} else {
				ui.Icon(c, iconFlame).Size(11, 11).TextColor(colorInkSoft)
			}
		}).OnClick(func() { a.togglePriority(t.ID) })
	})
}

func (a *app) draftRow(c *ui.Context, later *[]func()) {
	row := ui.Row(c.Key("draft")).Height(rowHeight).Padding(0, 6, 0, 4).Radius(rowRadius).Background(paper.rowHover)
	// Clicking away is the same as pressing Enter: keep what was typed, drop
	// an empty field.
	if row.PressedOutside() {
		*later = append(*later, a.commitNewTask)
	}
	row.Children(func() {
		// Stands in for the grip column so the field lines up with the rows.
		ui.Box(c).Width(12)
		ui.Row(c).Grow(1).MinWidth(0).Padding(0, 0, 0, 4).Children(func() {
			ui.TextInputBase(c, &a.draft).Placeholder("记一笔，回车保存").Grow(1).MinWidth(0).
				FontSize(13).AutoFocus().Label("新任务").OnSubmit(a.commitNewTask)
		})
	})
}

// historyPopover hangs under the history button. 272×300 is a wish, not a
// size: at the smallest window it would run off the edges, so it takes
// whichever is smaller. The scrim below the chrome swallows clicks, so a click
// meant to close the popover cannot also land on a row.
func (a *app) historyPopover(c *ui.Context, anchor ui.Element) {
	w, h := c.Size()
	ui.Overlay(c, func() {
		ui.Box(c).Absolute().Top(chromeHeight).Left(0).Right(0).Bottom(0).OnClick(a.toggleHistory)

		panel := ui.Column(c).Absolute().Top(chromeHeight+2).Right(gutter).
			Width(min(272, w-2*gutter)).MaxHeight(min(300, h-chromeHeight-2-gutter)).
			Background(colorRaised).Border(1, colorHairline).Radius(10).Clip()
		popoverShadow(panel).Children(func() {
			ui.Row(c).Padding(9, 10, 4, 10).Justify(ui.SpaceBetween).Children(func() {
				ui.Text(c, "已完成").FontSize(11.5).FontWeight(600).TextColor(colorInkSoft)
				if len(a.completed) > 0 {
					a.clearButton(c)
				}
			})
			if len(a.completed) == 0 {
				ui.Text(c, "勾掉的任务会留在这里").Padding(2, 12, 14, 12).FontSize(11.5).TextColor(colorInkFaint)
				return
			}
			ui.Scroll(c).Shrink(1).MinHeight(0).Padding(2, 6, 6, 6).Gap(2).Children(func() {
				for _, t := range a.completed {
					a.completedRow(c.Key(t.ID), t)
				}
			})
		})
	})
}

// clearButton cannot be undone, so the first click only arms it and says
// what the second will do. It disarms on a timer and when the pointer leaves.
func (a *app) clearButton(c *ui.Context) {
	b := ui.Row(c).Height(19).Padding(0, 5).Radius(4).Center().Cursor(ui.CursorPointer).Label("清空已完成")
	switch {
	case b.Pressed():
		b.Background(popover.actionBgActive)
	case b.Hovered():
		b.Background(popover.actionBg)
	case a.confirmClear:
		a.confirmClear = false
	}
	b.OnClick(a.clearCompleted).Children(func() {
		if a.confirmClear {
			ui.Text(c, "再点一次清空").FontSize(11).FontWeight(600).TextColor(colorPriority)
		} else {
			ui.Icon(c, iconTrash).Size(11, 11).TextColor(colorInkSoft)
		}
	})
}

func (a *app) completedRow(c *ui.Context, t store.Task) {
	row := ui.Row(c).Height(30).Padding(0, 6).Radius(5).Justify(ui.SpaceBetween)
	hovered := row.Hovered()
	armed := a.confirmDelete == t.ID
	if hovered {
		row.Background(popover.rowHover)
	} else if armed {
		a.confirmDelete, armed = "", false
	}
	row.Children(func() {
		ui.Row(c).Grow(1).MinWidth(0).Gap(8).Children(func() {
			ui.Box(c).Size(16, 16).Radius(8).Border(1, colorInkFaint).Center().Children(func() {
				ui.Icon(c, iconCheck).Size(11, 11).TextColor(colorInkSoft)
			})
			ui.Text(c, t.Title).SingleLine().Grow(1).MinWidth(0).FontSize(12.5).TextColor(colorInkSoft).Strikethrough()
		})
		if !hovered {
			return
		}
		ui.Row(c).Gap(4).Children(func() {
			actionButton(c, "恢复", 19, popover, false, func() {
				ui.Icon(c, iconRotate).Size(10, 10).TextColor(colorInkSoft)
			}).OnClick(func() { a.restoreTask(t.ID) })
			del := actionButton(c, "删除", 19, popover, false, func() {
				color := colorInkSoft
				if armed {
					color = colorRaised
				}
				ui.Icon(c, iconTrash).Size(10, 10).TextColor(color)
			}).OnClick(func() { a.requestDelete(t.ID) })
			if armed {
				del.Background(colorPriority)
			}
		})
	})
}

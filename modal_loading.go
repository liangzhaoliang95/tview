package tview

import (
	"github.com/gdamore/tcell/v2"
)

// ModalLoading implements a modal window with a custom form.
type ModalLoading struct {
	*Modal
}

// NewModalLoading implements a modal that can take in a custom form.
func NewModalLoading(title string) *ModalLoading {
	m := ModalLoading{NewModal()}
	m.frame = NewFrame(m.form).SetBorders(0, 0, 0, 0, 0, 0)
	m.frame.SetBorder(true).
		SetBackgroundColor(tcell.ColorBlue).
		SetBorderPadding(1, 1, 1, 1)
	m.frame.SetTitle(title)
	m.frame.SetTitleColor(tcell.ColorAqua)
	m.focus = m.form.focus

	return &m
}

// Draw draws this primitive onto the screen.
func (m *ModalLoading) Draw(screen tcell.Screen) {
	// Calculate the width of this modal.
	screenWidth, screenHeight := screen.Size()
	width := screenWidth / 3

	// Reset the text and find out how wide it is.
	m.frame.Clear()
	lines := WordWrap(m.text, width)
	for _, line := range lines {
		m.frame.AddText(line, true, AlignCenter, m.textColor)
	}

	// Set the modal's position and size.
	height := len(lines) + len(m.form.items) + len(m.form.buttons) + 5
	width += 4
	x := (screenWidth - width) / 2
	y := (screenHeight - height) / 2
	m.SetRect(x, y, width, height)

	// Draw the frame.
	m.frame.SetRect(x, y, width, height)
	m.frame.Draw(screen)
}

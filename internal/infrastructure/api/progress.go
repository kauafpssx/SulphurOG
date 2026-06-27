package api

import (
	"fmt"
	"os/exec"
	"strings"
)

type Progress struct {
	total    int64
	current  int64
	filename string
}

func NewProgress(filename string, total int64) *Progress {
	return &Progress{
		total:    total,
		filename: filename,
	}
}

func (p *Progress) Update(current int64) {
	p.current = current
	p.render()
}

func (p *Progress) render() {
	if p.total <= 0 {
		return
	}

	percent := float64(p.current) / float64(p.total) * 100
	barWidth := 30
	filled := int(percent / 100 * float64(barWidth))
	if filled > barWidth {
		filled = barWidth
	}

	bar := strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled)

	currentMB := float64(p.current) / 1024 / 1024
	totalMB := float64(p.total) / 1024 / 1024

	line := fmt.Sprintf("\r  %s %.1f/%.1fMB [%s] %.0f%%", p.filename, currentMB, totalMB, bar, percent)
	fmt.Print(line)
}

func (p *Progress) Done() {
	fmt.Println()
}

func ShowSpinner(title string) func() {
	cmd := exec.Command("gum", "spin", "--spinner", "dot", "--title", title)
	cmd.Start()
	return func() {
		cmd.Process.Kill()
	}
}

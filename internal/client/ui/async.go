package ui

import (
	"io"
	"os"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"golang.org/x/term"

	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
)

// RunWithSpinner 显示 spinner 执行 action；out 非 TTY 时静默执行（自动降级）
func RunWithSpinner(in io.Reader, out io.Writer, title string, action func() error) error {
	if !isTerminal(out) {
		return action()
	}
	model := &spinModel{spinner: spinner.New(spinner.WithSpinner(spinner.Dot)), title: title}
	program := tea.NewProgram(model, tea.WithInput(in), tea.WithOutput(out))
	go func() {
		program.Send(actionDoneMsg{err: action()})
	}()
	finalModel, err := program.Run()
	if err != nil {
		return ierr.Wrap(ierr.ErrInternal, err, "run spinner program")
	}
	return finalModel.(*spinModel).err
}

type actionDoneMsg struct{ err error }

type spinModel struct {
	spinner spinner.Model
	title   string
	err     error
}

func (m *spinModel) Init() tea.Cmd {
	return m.spinner.Tick
}

func (m *spinModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case actionDoneMsg:
		m.err = msg.err
		return m, tea.Quit
	case tea.KeyMsg:
		if msg.Type == tea.KeyCtrlC {
			m.err = ierr.New(ierr.ErrValidation, "aborted by user")
			return m, tea.Quit
		}
	}
	var cmd tea.Cmd
	m.spinner, cmd = m.spinner.Update(msg)
	return m, cmd
}

func (m *spinModel) View() string {
	return m.spinner.View() + " " + lipgloss.NewStyle().Foreground(colorMuted).Render(m.title)
}

func isTerminal(w io.Writer) bool {
	file, ok := w.(*os.File)
	return ok && term.IsTerminal(int(file.Fd()))
}

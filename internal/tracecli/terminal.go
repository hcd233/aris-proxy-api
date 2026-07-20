package tracecli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/hcd233/aris-proxy-api/internal/common/ierr"
	"golang.org/x/term"
)

type Terminal interface {
	Interactive() bool
	ReadLine(prompt string) (string, error)
	ReadSecret(prompt string) (string, error)
	WriteLine(line string)
}

type stdioTerminal struct {
	in     *os.File
	out    io.Writer
	reader *bufio.Reader
}

func NewTerminal() Terminal {
	return &stdioTerminal{in: os.Stdin, out: os.Stdout, reader: bufio.NewReader(os.Stdin)}
}

func (t *stdioTerminal) Interactive() bool {
	out, ok := t.out.(*os.File)
	return ok && term.IsTerminal(int(t.in.Fd())) && term.IsTerminal(int(out.Fd()))
}

func (t *stdioTerminal) ReadLine(prompt string) (string, error) {
	if _, err := fmt.Fprint(t.out, prompt); err != nil {
		return "", ierr.Wrap(ierr.ErrInternal, err, "write terminal prompt")
	}
	line, err := t.reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return "", ierr.Wrap(ierr.ErrInternal, err, "read terminal input")
	}
	return strings.TrimSpace(line), nil
}

func (t *stdioTerminal) ReadSecret(prompt string) (string, error) {
	if _, err := fmt.Fprint(t.out, prompt); err != nil {
		return "", ierr.Wrap(ierr.ErrInternal, err, "write secret prompt")
	}
	secret, err := term.ReadPassword(int(t.in.Fd()))
	if err != nil {
		return "", ierr.Wrap(ierr.ErrInternal, err, "read secret input")
	}
	_, _ = fmt.Fprintln(t.out) //nolint:errcheck // best-effort newline
	return strings.TrimSpace(string(secret)), nil
}

func (t *stdioTerminal) WriteLine(line string) {
	_, _ = fmt.Fprintln(t.out, line) //nolint:errcheck // best-effort write
}

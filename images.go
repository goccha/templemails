package templemails

import (
	"bufio"
	"github.com/goccha/templates/tmpl"
	"gopkg.in/mail.v2"
	"io"
	"os"
)

const (
	Images = iota
)

type EmbeddedImages struct {
	Images []EmbeddedImage `json:"images"`
}

type EmbeddedImage struct {
	Name     string             `json:"name"`
	Path     string             `json:"path"`
	Settings []mail.FileSetting `json:"-"`
}

func (img *EmbeddedImage) FilePath() string {
	return tmpl.GetFullPath(img.Path)
}
func (img *EmbeddedImage) Open() (io.Reader, error) {
	if f, err := os.Open(img.FilePath()); err != nil {
		return nil, err
	} else {
		return bufio.NewReader(f), nil
	}
}

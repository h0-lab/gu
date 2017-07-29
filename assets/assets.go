package assets

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"sync"
	"text/template"

	"github.com/gu-io/gu/assets/data"
	"github.com/influx6/moz/gen"
)

var (
	bufferPool = sync.Pool{
		New: func() interface{} {
			return &bytes.Buffer{}
		},
	}
)

// WriteDirective defines a type which defines a directive with details of the
// content to be written to and the original path and abspath of it's origin.
type WriteDirective struct {
	Writer        io.WriterTo
	OriginPath    string
	OriginAbsPath string
}

// Read will copy directives writer into a content buffer and returns the giving string
// representation of that data, content will be gzipped.
func (directive WriteDirective) Read() (string, error) {
	buffer := bufferPool.Get().(*bytes.Buffer)

	defer buffer.Reset()
	defer bufferPool.Put(buffer)

	if _, err := directive.Writer.WriteTo(gzip.NewWriter(buffer)); err != nil && err != io.EOF {
		return buffer.String(), err
	}

	return buffer.String(), nil
}

// Packer exposes a interface which exposes methods for validating the type of files
// it supports and a method to appropriately pack the FileStatments as desired
// into the given endpoint directory.
type Packer interface {
	Pack(files []FileStatement, dir DirStatement) ([]WriteDirective, error)
}

// Webpack defines the core structure for handling bundling of different assets
// using registered packers.
type Webpack struct {
	defaultPacker Packer
	packers       map[string]Packer
}

// New returns a new instance of the Webpack.
func New(defaultPacker Packer) *Webpack {
	return &Webpack{
		defaultPacker: defaultPacker,
		packers:       make(map[string]Packer, 0),
	}
}

// Register adds the Packer to manage the building of giving exensions.
func (w *Webpack) Register(ext string, packer Packer) {
	w.packers[ext] = packer
}

// Build runs through the directory pull all files and runs them through the
// packers to service each files by extension and returns a slice of all
// WriteDirective for final processing.
func (w *Webpack) Build(dir string, doGoSources bool) (map[string][]WriteDirective, error) {
	statement, err := GetDirStatement(dir, doGoSources)
	if err != nil {
		return nil, err
	}

	var wd map[string][]WriteDirective

	for ext, fileStatement := range statement.FilesByExt {
		packer, ok := w.packers[ext]
		if !ok && w.defaultPacker == nil {
			return wd, fmt.Errorf("No Packer provided to handle files with %q extension", ext)
		}

		var derr error
		var directives []WriteDirective

		if w.defaultPacker != nil && !ok {
			directives, derr = w.defaultPacker.Pack(fileStatement, statement)
		} else {
			directives, derr = packer.Pack(fileStatement, statement)
		}

		if derr != nil {
			return wd, err
		}

		wd[ext] = directives
	}

	return wd, nil
}

// Compile returns a io.WriterTo which contains a complete source of all assets
// generated and stored inside a io.WriteTo which will contain the go source excluding
// the package declaration so has to allow you write the contents into the package
// you wish.
func (w *Webpack) Compile(dir string, doGoSources bool) (io.WriterTo, error) {
	directives, err := w.Build(dir, doGoSources)
	if err != nil {
		return nil, err
	}

	content := gen.Block(
		gen.SourceTextWith(
			string(data.Must("packed.tml")),
			template.FuncMap{},
			struct {
				Dir        string
				Directives map[string][]WriteDirective
			}{
				Dir:        dir,
				Directives: directives,
			},
		),
	)

	return content, nil
}

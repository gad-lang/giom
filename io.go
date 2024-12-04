package giom

import "io"

type writer struct {
	io.Writer
	len  int
	data string
}

func (w *writer) Write(p []byte) (i int, err error) {
	w.data += string(p)
	i, err = w.Writer.Write(p)
	w.len += i
	return
}

package io

type readWriteCloserFunc struct {
	Reader
	Writer
	Closer
}

func NewReadWriteCloserFunc(reader Reader, writer Writer, closer Closer) ReadWriteCloser {
	return readWriteCloserFunc{
		Reader: reader,
		Writer: writer,
		Closer: closer,
	}
}

type readCloserFunc struct {
	Reader
	Closer
}

func NewReadCloserFunc(reader Reader, closer Closer) ReadCloser {
	return readCloserFunc{
		Reader: reader,
		Closer: closer,
	}
}

type readWriterFunc struct {
	Reader
	Writer
}

func NewReadWriterFunc(reader Reader, writer Writer) ReadWriter {
	return readWriterFunc{
		Reader: reader,
		Writer: writer,
	}
}

type CloseFunc = func() error

type closeFunc struct {
	closer CloseFunc
}

func (c closeFunc) Close() error {
	return c.closer()
}

func NewCloserFunc(closer CloseFunc) Closer {
	return closeFunc{
		closer: closer,
	}
}

type WriteFunc = func([]byte) (int, error)

type writerFunc struct {
	writer WriteFunc
}

func (w writerFunc) Write(p []byte) (int, error) {
	return w.writer(p)
}

func NewWriterFunc(writer WriteFunc) Writer {
	return writerFunc{
		writer: writer,
	}
}

type ReadFunc = func([]byte) (int, error)

type readerFunc struct {
	reader ReadFunc
}

func (r readerFunc) Read(p []byte) (int, error) {
	return r.reader(p)
}

func NewReaderFunc(reader ReadFunc) Reader {
	return readerFunc{
		reader: reader,
	}
}

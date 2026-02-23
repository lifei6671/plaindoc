package worker

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

var errCodecLineTooLarge = errors.New("ssr worker protocol line too large")

// stdioCodec 基于 JSONL（每行一个 JSON）实现进程间协议读写。
type stdioCodec struct {
	reader       *bufio.Reader
	writer       io.Writer
	maxReadBytes int64
}

func newStdioCodec(reader io.Reader, writer io.Writer, maxReadBytes int64) *stdioCodec {
	return &stdioCodec{
		reader:       bufio.NewReader(reader),
		writer:       writer,
		maxReadBytes: maxReadBytes,
	}
}

func (codec *stdioCodec) WriteJSON(value any) error {
	if codec == nil {
		return errors.New("ssr worker codec is nil")
	}

	payloadBytes, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("marshal ssr worker payload: %w", err)
	}
	payloadBytes = append(payloadBytes, '\n')

	if _, err := codec.writer.Write(payloadBytes); err != nil {
		return fmt.Errorf("write ssr worker payload: %w", err)
	}
	return nil
}

func (codec *stdioCodec) ReadJSON(value any) error {
	if codec == nil {
		return errors.New("ssr worker codec is nil")
	}

	lineBytes, err := codec.reader.ReadBytes('\n')
	if err != nil {
		return fmt.Errorf("read ssr worker payload: %w", err)
	}
	if codec.maxReadBytes > 0 && int64(len(lineBytes)) > codec.maxReadBytes {
		return fmt.Errorf("%w: %d > %d", errCodecLineTooLarge, len(lineBytes), codec.maxReadBytes)
	}

	if err := json.Unmarshal(lineBytes, value); err != nil {
		return fmt.Errorf("decode ssr worker payload: %w", err)
	}
	return nil
}

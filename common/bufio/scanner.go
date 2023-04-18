package bufio

import (
	"v2ray.com/core/common/io"
)

func ReadLine(file io.Reader) ([]string, error) {
	str := make([]string, 0)

	scanner := NewScanner(file)
	for scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return nil, err
		}

		str = append(str, scanner.Text())
	}

	return str, nil
}

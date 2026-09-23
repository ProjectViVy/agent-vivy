//go:build windows

package runtime

import "errors"

func mkfifoForTest(path string) error {
	return errors.New("fifo fixtures unsupported on windows")
}

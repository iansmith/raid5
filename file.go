package raid5

import (
	"errors"
	"hash"
	"os"
	"path/filepath"
	"strings"
)

//Public API to this type is in upper case.
type raid5File struct {
	f1, f2 *os.File
	parity *os.File

	//support for overriding in tests
	blockWriter func([]byte) error
	writer      func([]byte) error
}

var (
	WRONG_SIZE_WRITE = errors.New("we are assuming that writes to a disk always write the full set of bytes")
)

const (
	BLOCK_SIZE = 0x10000
	HALF_BLOCK = BLOCK_SIZE >> 1
)

//create a file. directories are "out of band" information not really
//part of the public api
func CreateFile(dir1, dir2, parityDir, name string) (*raid5File, error) {
	if BLOCK_SIZE%2 != 0 {
		panic("bad block size! block size must be even!")
	}
	f1, err := os.Create(filepath.Join(dir1, name))
	if err != nil {
		return nil, err
	}
	f2, err := os.Create(filepath.Join(dir2, name))
	if err != nil {
		f1.Close()
		return nil, err
	}
	parity, err := os.Create(filepath.Join(parityDir, name))
	if err != nil {
		f1.Close()
		f2.Close()
		return nil, err
	}
	result := &raid5File{
		f1:     f1,
		f2:     f2,
		parity: parity,
	}

	result.writer = result.write
	result.blockWriter = result.writeSingleBlock
	return result, nil //no error
}

//close a file, not clear how to return the erorrs.  we are going to
//try to close all the files first and then deal with errors
func (self *raid5File) Close() error {
	e1 := self.f1.Close()
	e2 := self.f2.Close()
	e3 := self.parity.Close()

	for _, e := range []error{e1, e2, e3} {
		if e != nil {
			return e
		}
	}
	return nil
}

//write exactly one block
func (self *raid5File) writeSingleBlock(data []byte) error {
	if len(data) != BLOCK_SIZE {
		panic("unexpected size of block in WriteBlock!")
	}
	//compute parity via XOR
	parity := make([]byte, HALF_BLOCK)
	for i, _ := range parity {
		parity[i] = data[i] ^ data[i+HALF_BLOCK]
	}

	for which, blob := range [][]byte{data[:HALF_BLOCK], data[HALF_BLOCK:], parity} {
		var n int
		var err error
		switch which {
		case 0:
			n, err = self.f1.Write(blob)
		case 1:
			n, err = self.f2.Write(blob)
		case 2:
			n, err = self.parity.Write(blob)
		}
		if n != HALF_BLOCK || err != nil {
			if err != nil {
				return err
			}
			return WRONG_SIZE_WRITE
		}
	}
	//everything is ok
	return nil
}

//write any size of data blob, padding the end to fit exactly in the
//block size
func (self *raid5File) write(data []byte) error {
	curr := 0
	for curr < len(data) {
		//can we write a whole block?
		if len(data)-curr > BLOCK_SIZE {
			if err := self.blockWrite(data[curr : curr+BLOCK_SIZE]); err != nil {
				return err
			}
		} else {
			//pad with zeros as necessary
			padding := make([]byte, BLOCK_SIZE)
			for i := 0; i < BLOCK_SIZE; i++ {
				//if statement not very satisfying as it avoids burstish
				//writes of zeros but this is easier to reason about correctness
				if curr+i < len(data) {
					padding[i] = data[curr+i]
				} else {
					padding[i] = 0x00
				}
			}

			if err := self.blockWrite(padding); err != nil {
				return err
			}
		}
		curr += BLOCK_SIZE
	}
	return nil
}

//write defaults to calling the standard implementation
func (self *raid5File) Write(data []byte) error {
	return self.writer(data)
}

//blockWrite defaults to calling the standard implementation
func (self *raid5File) blockWrite(data []byte) error {
	return self.blockWriter(data)
}

//hide the length of the file and the md5hash in the name
func encodeMetadata(name string, len int64, hash []byte) string {
	if strings.Index(name, "$") != -1 {
		panic("illegal character in filename!")
	}
	if name == "" {
		panic("empty filenames are nonsense")
	}
	return fmt.Sprintf("%s$%x$%x", name, len, hash)
}

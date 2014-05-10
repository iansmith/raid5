package raid5

import (
	"crypto/md5"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

//Public API to this type is in upper case.
type raid5File struct {
	f1, f2 *os.File
	parity *os.File

	startingName string
	//after a WriteAndClose() this holds the underlying FS name
	finalName string

	//support for overriding in tests
	blockWriter func([]byte) error
	writer      func([]byte) (int64, []byte, error)
}

var (
	WRONG_SIZE_WRITE = errors.New("we are assuming that writes to a disk always write the full set of bytes")
)

const (
	BLOCK_SIZE           = 0x10000
	HALF_BLOCK           = BLOCK_SIZE >> 1
	HASH_LENGTH_IN_ASCII = 32
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
		startingName: name,
		f1:           f1,
		f2:           f2,
		parity:       parity,
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
//block size.  note that the extra values returned here are primarily
//for the code that is renaming the file to encode extra things in the
//name.
func (self *raid5File) write(data []byte) (int64, []byte, error) {
	curr := 0
	h := md5.New()
	var mostRecent []byte

	for curr < len(data) {
		//can we write a whole block?
		if len(data)-curr > BLOCK_SIZE {
			if err := self.blockWrite(data[curr : curr+BLOCK_SIZE]); err != nil {
				return 0, nil, err
			}
			mostRecent = h.Sum(data[curr : curr+BLOCK_SIZE])
		} else {
			//pad with zeros as necessary
			padding := make([]byte, BLOCK_SIZE)
			mostRecent = h.Sum(data[curr:]) //hash does not include zeros!
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
				return 0, nil, err
			}
		}
		curr += BLOCK_SIZE
	}
	return int64(len(data)), mostRecent, nil
}

//WriteAndClose defaults to calling the standard implementation, which is
//just write.
func (self *raid5File) WriteAndClose(data []byte) (int64, []byte, error) {
	l, h, err := self.writer(data)
	if err != nil {
		return l, h, err // give up
	}
	if err := self.Close(); err != nil {
		return 0, nil, err //is there something more useful to do here?
	}
	self.finalName = encodeMetadata(name, l, h)
	parentF1 := filepath.Dir(self.f1.Name())
	parentF2 := filepath.Dir(self.f2.Name())
	parentParity := filepath.Dir(self.parity.Name())

	//rename is pretty cheap in most systems
	//we are ignoring collisions here because it's both irrelevant and
	//in a better implementation it would be helpful to "unify" files
	//with identical content (which is the case here)
	if err := os.Rename(self.f1.Name(), filepath.Join(parentF1, self.finalName)); err != nil {
		return 0, nil, err
	}
	if err := os.Rename(self.f2.Name(), filepath.Join(parentF2, self.finalName)); err != nil {
		return 0, nil, err
	}
	if err := os.Rename(self.parity.Name(), filepath.Join(parentParity, self.finalName)); err != nil {
		return 0, nil, err
	}
	return l, h, nil
}

//blockWrite defaults to calling the standard implementation
func (self *raid5File) blockWrite(data []byte) error {
	return self.blockWriter(data)
}

//hide the length of the file and the md5hash in the name
func encodeMetadata(name string, l int64, hash []byte) string {
	if strings.Index(name, "$") != -1 {
		panic("illegal character in filename!")
	}
	if name == "" {
		panic("empty filenames are nonsense")
	}
	return fmt.Sprintf("%s$%x$%x", name, l, hash)
}

//hide the length of the file and the md5hash in the name
func decodeMetadata(name string) (string, int64, []byte) {
	pieces := strings.Split(name, "$")
	if len(pieces) != 3 {
		panic("badly encoded name sent to decode!")
	}
	l, err := strconv.ParseInt(pieces[1], 10, 64)
	if err != nil {
		panic("badly encoded name sent to decode (base 10 expected for length)")
	}
	if len(pieces[2]) != HASH_LENGTH_IN_ASCII {
		panic("badly encoded name sent to decode (base 16 hash is wrong length)")
	}
	h := make([]byte, HASH_LENGTH_IN_ASCII/2)
	for i := 0; i < len(pieces[2]); i += 2 {
		b, err := strconv.ParseInt(pieces[2][i:i+2], 16, 64)
		if err != nil {
			panic("badly encoded name sent to decode (base 16 hash)")
		}
		h[i/2] = byte(b) //yes, this is safe
	}
	return pieces[0], l, h
}

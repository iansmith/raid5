package raid5

import (
	"crypto/md5"
	"errors"
	"fmt"
	"log"
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

	expectedLen  int64
	expectedHash []byte

	//support for overriding in tests
	blockWriter func([]byte) error
	writer      func([]byte) (int64, []byte, error)
}

var (
	WRONG_SIZE = errors.New("we are assuming that writes to a disk always read/write the full set of bytes")
)

const (
	BLOCK_SIZE           = 0x10000
	HALF_BLOCK           = BLOCK_SIZE >> 1
	HASH_LENGTH_IN_ASCII = 32
)

func (self *raid5File) Size() int64 {
	return self.expectedLen
}

//create a file. directories are "out of band" information not really
//part of the public api.  note that this will return an error if the
//file already exists.
func CreateFile(dir1, dir2, parityDir, name string) (*raid5File, error) {
	if BLOCK_SIZE%2 != 0 {
		panic("bad block size! block size must be even!")
	}

	_, err1 := os.Open(filepath.Join(dir1, name))
	_, err2 := os.Open(filepath.Join(dir2, name))
	_, err3 := os.Open(filepath.Join(parityDir, name))

	//should we be doing voting here?
	if err1 == nil || err2 == nil || err3 == nil {
		return nil, os.ErrExist
	}

	//should we be doing voting?
	if err1 != nil || err2 != nil || err3 != nil {
		if !os.IsNotExist(err1) || !os.IsNotExist(err2) || !os.IsNotExist(err3) {
			return nil, err1 //maybe should panic?
		}
		//we continue because we WANT there to be the three not exist
		//errors
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
			return WRONG_SIZE
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

	for curr < len(data) {
		//can we write a whole block?
		if len(data)-curr > BLOCK_SIZE {
			if err := self.blockWrite(data[curr : curr+BLOCK_SIZE]); err != nil {
				return 0, nil, err
			}
			h.Write(data[curr : curr+BLOCK_SIZE])
		} else {
			//pad with zeros as necessary
			padding := make([]byte, BLOCK_SIZE)
			h.Write(data[curr:]) //hash does not include zeros!
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
	return int64(len(data)), h.Sum(nil), nil
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
	self.finalName = encodeMetadata(self.startingName, l, h)
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
	//XXX NOT ATOMIC! CONCURRENCY PROBLEM!!
	if err := os.Symlink(filepath.Join(parentF1, self.finalName), filepath.Join(parentF1, self.startingName)); err != nil {
		return 0, nil, err
	}
	if err := os.Symlink(filepath.Join(parentF2, self.finalName), filepath.Join(parentF2, self.startingName)); err != nil {
		return 0, nil, err
	}
	if err := os.Symlink(filepath.Join(parentParity, self.finalName), filepath.Join(parentParity, self.startingName)); err != nil {
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
	return fmt.Sprintf("%s$%d$%x", name, l, hash)
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

func OpenFile(d1, d2, parity, name string) (*raid5File, error) {
	//try to open all three files
	paths := []string{
		filepath.Join(d1, name),
		filepath.Join(d2, name),
		filepath.Join(parity, name),
	}

	f1, err1 := os.Open(paths[0])
	f2, err2 := os.Open(paths[1])
	p, err3 := os.Open(paths[2])

	ct := 0
	for i, err := range []error{err1, err2, err3} {
		if err == nil {
			ct++
			continue
		}
		if os.IsNotExist(err) {
			log.Printf("trying to recover from data missing in %s", paths[i])
			continue // we can maybe tolerate this error
		}
		//not clear: is this an error? if the disk is failing, it seems
		//like it is, so we error here
		return nil, err
	}
	if ct < 2 {
		return nil, os.ErrNotExist
	}
	//figure out how long the file is and its expected hash, we can use
	//one of the two parts
	link := filepath.Join(d1, name)
	if f1 == nil {
		link = filepath.Join(d2, name)
	}
	dest, linkErr := os.Readlink(link)
	if linkErr != nil {
		info, err := os.Stat(link)
		if err != nil {
			return nil, err
		}
		if info.Size() != 0 {
			return nil, linkErr
		}
		//zero sized file
		return &raid5File{
			f1:           f1,
			f2:           f2,
			parity:       p,
			startingName: name,
			finalName:    name,
		}, nil

	}
	_, l, hsh := decodeMetadata(dest)

	return &raid5File{
		f1:           f1,
		f2:           f2,
		parity:       p,
		startingName: name,
		finalName:    dest,
		expectedLen:  l,
		expectedHash: hsh,
	}, nil
}

func (self *raid5File) ReadFile(out []byte, offset int64) (int64, error) {

	//setup r1 and r2
	r1 := self.f1
	r2 := self.f2
	usingParity := false
	if r1 == nil || r2 == nil {
		usingParity = true
		//we know that the additional file we need is in p
		if r1 == nil {
			r1 = self.parity
		} else {
			r2 = self.parity
		}
	}

	//seek to the location requested
	if _, err := r1.Seek(offset, 0); err != nil {
		return 0, err
	}
	if _, err := r2.Seek(offset, 0); err != nil {
		return 0, err
	}

	curr := 0
	data1 := make([]byte, HALF_BLOCK)
	data2 := make([]byte, HALF_BLOCK)

	for curr < len(out) {
		n1, err := r1.Read(data1)
		if err != nil {
			return 0, err
		}
		n2, err := r2.Read(data2)
		if err != nil {
			return 0, err
		}
		if n1 != len(data1) {
			return 0, WRONG_SIZE
		}
		if n2 != len(data2) {
			return 0, WRONG_SIZE
		}
		if usingParity {
			for i := 0; i < HALF_BLOCK; i++ {
				recovered := data1[i] ^ data2[i]
				if self.f1 == nil {
					data1[i] = recovered
				} else {
					data2[i] = recovered
				}
			}
		}
		//data is recovered if necessary, so copy it out
		if int64(curr+HALF_BLOCK) > self.expectedLen {
			copy(out[curr:], data1[:self.expectedLen-int64(curr)])
		} else if int64(curr+BLOCK_SIZE) > self.expectedLen {
			copy(out[curr:], data1)
			copy(out[curr+HALF_BLOCK:], data2[:self.expectedLen-int64(curr+HALF_BLOCK)])
		} else {
			copy(out[curr:], data1)
			copy(out[curr+HALF_BLOCK:], data2)
		}

		//jump a block
		curr += BLOCK_SIZE
	}

	return self.expectedLen, nil
}

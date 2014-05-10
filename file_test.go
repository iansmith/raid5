package raid5

import (
	"io/ioutil"
	"math/rand"
	"os"
	"path/filepath"
	"testing"
)

func setupTestDirs(t *testing.T) (string, string, string) {
	data1, err := ioutil.TempDir("", "raid5")
	if err != nil {
		t.Fatalf("creating data dir: %v", err)
	}
	data2, err := ioutil.TempDir("", "raid5")
	if err != nil {
		t.Fatalf("creating data dir2: %v", err)
	}
	parity, err := ioutil.TempDir("", "raid5")
	if err != nil {
		t.Fatalf("creating parity dir: %v", err)
	}
	return data1, data2, parity
}

func destroyTestDirs(t *testing.T, data1, data2, parity string) {
	for _, dir := range []string{data1, data2, parity} {
		err := os.RemoveAll(dir)
		if err != nil {
			t.Fatalf("failed to remove dir %s: %v", dir, err)
		}
	}
}

func TestCreateFile(t *testing.T) {
	d1, d2, parity := setupTestDirs(t)
	defer destroyTestDirs(t, d1, d2, parity)

	name := "foobie"
	result, err := CreateFile(d1, d2, parity, name)
	if err != nil {
		t.Fatalf("failed to create files: %v", err)
	}

	//write random number of bytes into the files so we can be sure
	//these are leftover from some previous test, this is not done through
	//the API to raid5file because this is really testing the filenames
	size := rand.Intn(0x100)
	data := make([]byte, size)
	for _, file := range []*os.File{result.f1, result.f2, result.parity} {
		n, err := file.Write(data)
		if err != nil {
			t.Fatalf("failed writing test data: %v", err)
		}
		if n != size {
			t.Fatalf("failed in writing a few bytes? %d!=%d", n, size)
		}
	}

	//do the close so we are sure data is on the disk
	err = result.Close()
	if err != nil {
		t.Fatalf("failed to close files: %v", err)
	}

	//these tests may seem excessive but I need to be sure that the
	//files are going where I think they are because the strategy is
	//to just manipulate the string names once, and from there use
	//*open* files
	expected := []string{
		filepath.Join(d1, name),
		filepath.Join(d2, name),
		filepath.Join(parity, name),
	}

	for _, expectedPath := range expected {
		fp, err := os.Open(expectedPath)
		if err != nil {
			t.Errorf("failed to find file %s %v", expectedPath, err)
		}
		defer fp.Close()

		info, err := fp.Stat()
		if err != nil {
			t.Fatalf("unable to stat %v: %v", fp, err)
		}

		if info.Size() != int64(size) {
			t.Errorf("wrong number of bytes found, expected %d but got %d", size, info.Size())
		}

	}

}

func TestNiceSizedWrite(t *testing.T) {
	d1, d2, parity := setupTestDirs(t)
	defer destroyTestDirs(t, d1, d2, parity)

	name := "bletch"
	raid5, err := CreateFile(d1, d2, parity, name)
	if err != nil {
		t.Fatalf("failed to create raid5 files: %v", err)
	}

	simpleBlock := make([]byte, BLOCK_SIZE)
	for i, _ := range simpleBlock {
		//inner mod is because we want to start at 0 on HALF_BLOCK
		simpleBlock[i] = byte((i % HALF_BLOCK) % 0xf)
		if i >= HALF_BLOCK {
			simpleBlock[i] |= 0x80 //seems legit?
		}
	}

	//hitting the internal call, not through the API proper
	err = raid5.blockWrite(simpleBlock)
	if err != nil {
		t.Fatalf("failed to write bytes: %v", err)
	}

	err = raid5.Close()
	if err != nil {
		t.Fatalf("could not close the raid5 file: %v", err)
	}

	//read the files from the filesystem via the go primitives since this
	//is a test of the raid5 level code
	expected := []string{
		filepath.Join(d1, name),
		filepath.Join(d2, name),
		filepath.Join(parity, name),
	}

	testPred := []func(*testing.T, byte, int) bool{
		//first half
		func(t *testing.T, b byte, i int) bool {
			if b != byte(i%0xf) {
				t.Errorf("failed in part 1: expected %x but got %x", i%0xf, b)
				return false
			}
			return true
		},

		//second half
		func(t *testing.T, b byte, i int) bool {
			if b != byte(i%0xf)|byte(0x80) {
				t.Errorf("failed in part 2: expected %x but got %x", byte(i%0xf)|byte(0x80), b)
				return false
			}
			return true
		},

		//parity
		func(t *testing.T, b byte, i int) bool {
			xor := byte(i%0xf) ^ byte(i%0xf|0x80)
			if b != xor {
				t.Errorf("failed in parity: expected %x but got %x", xor, b)
				return false
			}
			return true
		},
	}
	for which, expectedPath := range expected {
		fp, err := os.Open(expectedPath)
		if err != nil {
			t.Fatalf("failed to open expected file %s : %v", expectedPath, err)
		}
		defer fp.Close()

		info, err := fp.Stat()
		if err != nil {
			t.Fatalf("failed to stat: %v", err)
		}

		//three files * half block size = 50% overhead (MEETS SPEC!)
		if info.Size() != HALF_BLOCK {
			t.Errorf("wrong number of bytes written, expected %d but got %d", HALF_BLOCK, info.Size())
		}

		//this depends on the writing code writing the first half in
		//f1 and the second half in f2 rather than interleaving
		buffer := make([]byte, HALF_BLOCK)
		n, err := fp.Read(buffer)
		if err != nil || n != HALF_BLOCK {
			t.Fatalf("could not read first data blob: (%v) %v", n == HALF_BLOCK, err)
		}
		//test the bytes
		for i, b := range buffer {
			if !testPred[which](t, b, i) {
				t.Logf("failed on byte %d of section %d", i, which)
				break //no sense reporting a bunch of errors
			}
		}
	}
}

func TestUglySizedWrite(t *testing.T) {
	d1, d2, parity := setupTestDirs(t)
	defer destroyTestDirs(t, d1, d2, parity)

	name := "bletch"
	raid5, err := CreateFile(d1, d2, parity, name)
	if err != nil {
		t.Fatalf("failed to create raid5 files: %v", err)
	}

	sizes_to_test := []int{0x0, 0x1, 0xffff - 1, 0xffff + 1,
		2 * 0xffff, 2*0xffff + 1, 0xffff + 5*rand.Intn(0xffff)}

	countCalls := 0
	//setup the test rigging
	raid5.blockWriter = func(data []byte) error {
		if len(data) != BLOCK_SIZE {
			t.Logf("len data is %x", len(data))
			panic("wrong sized block passed in!")
		}
		countCalls++
		return nil
	}

	//establish test data
	testData := make([][]byte, len(sizes_to_test))
	for i, _ := range testData {
		testData[i] = make([]byte, sizes_to_test[i])
		for j, _ := range testData[i] {
			testData[i][j] = byte(j % 0x7f) //never set the high bit
		}
	}

	//run each test, computing the number of times we should be
	//writing to WriteBlock
	for _, data := range testData {
		blocks := len(data) / BLOCK_SIZE
		if len(data)%BLOCK_SIZE != 0 {
			//didn't fit exactly, so add extra block for remainder
			//of the data
			blocks++
		}

		countCalls = 0
		err := raid5.Write(data)
		if err != nil {
			t.Fatalf("unable to write the data blob: %v", err)
		}
		if countCalls != blocks {
			t.Errorf("wrong number of blocks written (expected %d but got %d) for size %d",
				blocks, countCalls, len(data))
		}
	}

}

package shutil

import (
	"bytes"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"testing"

	. "github.com/onsi/gomega"
)

const testdir = "_test"

// Testing utility functions

func filesMatch(src, dst string) (bool, error) {
	srcContents, err := ioutil.ReadFile(src)
	if err != nil {
		return false, err
	}

	dstContents, err := ioutil.ReadFile(dst)
	if err != nil {
		return false, err
	}

	if !bytes.Equal(srcContents, dstContents) {
		return false, nil
	}
	return true, nil
}

func setup() {
	cmd := exec.Command("cp", "-a", "test", testdir)
	cmd.Run()
}

func teardown() {
	cmd := exec.Command("rm", "-rf", testdir)
	cmd.Run()
}

func makeTestPath(p string) string {
	return path.Join(testdir, p)
}

// CopyFile Tests

func TestCopyFile(t *testing.T) {
	setup()
	t.Cleanup(teardown)
	g := NewWithT(t)

	src1 := makeTestPath("testfile")
	src2 := makeTestPath("testfile2")
	dst := makeTestPath("testfile3")

	g.Expect(CopyFile(src1, dst, false)).To(Succeed())
	g.Expect(filesMatch(src1, dst)).To(BeTrue())

	g.Expect(CopyFile(src2, dst, false)).To(Succeed())
	g.Expect(filesMatch(src2, dst)).To(BeTrue())
}

// Copy Tests

func TestCopySameFileError(t *testing.T) {
	setup()
	t.Cleanup(teardown)
	g := NewWithT(t)

	src := makeTestPath("testfile")
	_, err := Copy(src, src, false)
	g.Expect(err).Should(MatchError(&SameFileError{src, src}))
}

func TestCopy(t *testing.T) {
	setup()
	t.Cleanup(teardown)
	g := NewWithT(t)

	src1 := makeTestPath("testfile")
	src2 := makeTestPath("testfile2")
	dst := makeTestPath("testfile3")

	g.Expect(Copy(src1, dst, false)).To(Equal(dst))
	g.Expect(filesMatch(src1, dst)).To(BeTrue())

	g.Expect(Copy(src2, dst, false)).To(Equal(dst))
	g.Expect(filesMatch(src2, dst)).To(BeTrue())
}

// CopyTree tests

func TestCopyTree(t *testing.T) {
	setup()
	t.Cleanup(teardown)
	g := NewWithT(t)

	src := makeTestPath("testdir")
	dst := makeTestPath("testdir3")

	srcFile := makeTestPath("testdir/file1")
	dstFile := makeTestPath("testdir3/file1")

	g.Expect(CopyTree(src, dst, nil)).To(Succeed())
	g.Expect(filesMatch(srcFile, dstFile)).To(BeTrue())
}

func TestCopyTreeMissingSource(t *testing.T) {
	setup()
	t.Cleanup(teardown)
	g := NewWithT(t)

	g.Expect(CopyTree(makeTestPath("testdir0"), makeTestPath("testdir3"), nil)).Should(HaveOccurred())
}

func TestCopyTreeSourceFile(t *testing.T) {
	setup()
	t.Cleanup(teardown)
	g := NewWithT(t)

	g.Expect(CopyTree(makeTestPath("testfile"), makeTestPath("testdir3"), nil)).Should(HaveOccurred())
}

// Move tests

func TestSimpleMove(t *testing.T) {
	setup()
	t.Cleanup(teardown)
	g := NewWithT(t)

	src := makeTestPath("testdir")
	dst := makeTestPath("testdir2")

	// Should succeed and return destination
	g.Expect(Move(src, dst, nil)).To(Equal(dst))

	// Source directory should not exist
	_, err := os.Stat(src)
	g.Expect(err).Should(HaveOccurred())

}

func TestMoveExisting(t *testing.T) {
	setup()
	t.Cleanup(teardown)
	g := NewWithT(t)

	src := makeTestPath("testdir")
	dst := testdir

	// Should fail because target exists already
	_, err := Move(src, dst, nil)
	g.Expect(err).Should(HaveOccurred())
}

// Private function tests

func TestDestInSrcTrue(t *testing.T) {
	g := NewWithT(t)

	g.Expect(destinsrc("_test", "_test/testdir/")).To(BeTrue())
	g.Expect(destinsrc("_test/", "_test/testdir")).To(BeTrue())
	g.Expect(destinsrc("_test/", "_test/testdir/")).To(BeTrue())
}

func TestDestInSrcFalse(t *testing.T) {
	g := NewWithT(t)

	g.Expect(destinsrc("_test/testdir", "_test/empty/")).To(BeFalse())
	g.Expect(destinsrc("_test/testdir/", "_test/empty")).To(BeFalse())
	g.Expect(destinsrc("_test/testdir/", "_test/empty/")).To(BeFalse())
}

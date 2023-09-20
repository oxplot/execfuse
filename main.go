package main

import (
	"bytes"
	"context"
	"flag"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"

	"bazil.org/fuse"
	"bazil.org/fuse/fs"
)

var (
	srcDir = flag.String("src", "", "src directory")
	mntDir = flag.String("mount", "", "mount directory")
	script = flag.String("script", "", "script to run to transform files")

	tree = &fs.Tree{}
)

type node struct {
	inode uint64
	path  string
}

// Attr returns fuse.Attr that describes the file.
func (t *node) Attr(ctx context.Context, a *fuse.Attr) error {
	st, err := os.Stat(t.path)
	if err != nil {
		return err
	}
	*a = fuse.Attr{
		Valid:  0,
		Inode:  t.inode,
		Size:   1000000000,
		Mode:   st.Mode(),
		Blocks: 1,
		Atime:  st.ModTime(),
		Mtime:  st.ModTime(),
		Ctime:  st.ModTime(),
		Nlink:  1,
		Uid:    uint32(os.Getuid()),
		Gid:    uint32(os.Getgid()),
	}
	return nil
}

func (t *node) Open(ctx context.Context, req *fuse.OpenRequest, resp *fuse.OpenResponse) (fs.Handle, error) {
	return t, nil
}

func (t *node) ReadAll(ctx context.Context) ([]byte, error) {
	f, err := os.Open(t.path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	b := &bytes.Buffer{}
	cmd := exec.Command("/bin/sh", "-c", *script)
	cmd.Stdin = f
	cmd.Stdout = b
	if err := cmd.Run(); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

func main() {

	flag.Parse()

	if *srcDir == "" || *mntDir == "" || *script == "" {
		log.Fatal("src, mount and script are required")
	}

	d, err := os.ReadDir(*srcDir)
	if err != nil {
		log.Fatal(err)
	}
	for i, f := range d {
		if f.IsDir() {
			continue
		}
		tree.Add(f.Name(), &node{inode: uint64(i), path: filepath.Join(*srcDir, f.Name())})
	}

	c, err := fuse.Mount(*mntDir)
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()

	sigCh := make(chan os.Signal, 1)
	go func() {
		<-sigCh
		log.Print("unmounting")
		fuse.Unmount(*mntDir)
	}()
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	err = fs.Serve(c, tree)
	if err != nil {
		log.Fatal(err)
	}
}

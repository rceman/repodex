package store

import "path/filepath"

const dirName = ".repodex"

// Dir returns the base directory for Repodex data.
func Dir(root string) string {
	return filepath.Join(root, dirName)
}

func ConfigPath(root string) string {
	return filepath.Join(Dir(root), "config.json")
}

func IgnorePath(root string) string {
	return filepath.Join(Dir(root), "ignore")
}

func MetaPath(root string) string {
	return filepath.Join(Dir(root), "meta.json")
}

func FilesPath(root string) string {
	return filepath.Join(Dir(root), "files.dat")
}

func ChunksPath(root string) string {
	return filepath.Join(Dir(root), "chunks.dat")
}

func TermsPath(root string) string {
	return filepath.Join(Dir(root), "terms.dat")
}

func PostingsPath(root string) string {
	return filepath.Join(Dir(root), "postings.dat")
}

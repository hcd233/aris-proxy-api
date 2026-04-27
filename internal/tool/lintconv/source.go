package lintconv

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

type SourceFile struct {
	Path string
	File *ast.File
	Set  *token.FileSet
}

func loadSourceFiles(args []string) ([]SourceFile, []Diagnostic) {
	if len(args) == 0 {
		args = []string{"."}
	}
	var files []SourceFile
	var diagnostics []Diagnostic
	for _, arg := range args {
		paths, err := expandGoFiles(arg)
		if err != nil {
			diagnostics = append(diagnostics, Diagnostic{Rule: "source.load", Severity: SeverityError, Path: slashPath(arg), Line: 1, Message: err.Error()})
			continue
		}
		for _, path := range paths {
			set := token.NewFileSet()
			file, err := parser.ParseFile(set, path.abs, nil, parser.ParseComments)
			if err != nil {
				diagnostics = append(diagnostics, Diagnostic{Rule: "source.parse", Severity: SeverityError, Path: path.rel, Line: 1, Message: err.Error()})
				continue
			}
			files = append(files, SourceFile{Path: path.rel, File: file, Set: set})
		}
	}
	return files, diagnostics
}

type goFilePath struct {
	abs string
	rel string
}

func expandGoFiles(arg string) ([]goFilePath, error) {
	clean := filepath.Clean(strings.TrimSuffix(arg, "/..."))
	if clean == "..." {
		clean = "."
	}
	info, err := os.Stat(clean)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		if strings.HasSuffix(clean, ".go") {
			return []goFilePath{{abs: clean, rel: slashPath(clean)}}, nil
		}
		return nil, nil
	}
	rootAbs, err := filepath.Abs(clean)
	if err != nil {
		return nil, err
	}
	var paths []goFilePath
	err = filepath.WalkDir(rootAbs, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			name := entry.Name()
			if name == ".git" || name == ".worktrees" || name == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(entry.Name(), ".go") {
			rel, err := filepath.Rel(rootAbs, path)
			if err != nil {
				return err
			}
			paths = append(paths, goFilePath{abs: path, rel: slashPath(rel)})
		}
		return nil
	})
	return paths, err
}

func slashPath(path string) string {
	return filepath.ToSlash(filepath.Clean(path))
}

func (file SourceFile) line(pos token.Pos) int {
	if !pos.IsValid() {
		return 1
	}
	return file.Set.Position(pos).Line
}

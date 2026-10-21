package lintconv

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/hcd233/aris-proxy-api/internal/common/constant"
	"github.com/hcd233/aris-proxy-api/internal/common/enum"
)

type SourceFile struct {
	Path string
	File *ast.File
	Set  *token.FileSet
}

func loadSourceFiles(args []string) ([]SourceFile, []Diagnostic) {
	if len(args) == 0 {
		args = []string{constant.ConvCheckCurrentDir}
	}
	var files []SourceFile
	var diagnostics []Diagnostic
	for _, arg := range args {
		paths, err := expandGoFiles(arg)
		if err != nil {
			diagnostics = append(diagnostics, Diagnostic{Rule: constant.ConvCheckRuleSourceLoad, Severity: enum.SeverityError, Path: slashPath(arg), Line: 1, Message: err.Error()})
			continue
		}
		for _, path := range paths {
			set := token.NewFileSet()
			file, err := parser.ParseFile(set, path.abs, nil, parser.ParseComments)
			if err != nil {
				diagnostics = append(diagnostics, Diagnostic{Rule: constant.ConvCheckRuleSourceParse, Severity: enum.SeverityError, Path: path.rel, Line: 1, Message: err.Error()})
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
	clean := filepath.Clean(strings.TrimSuffix(arg, constant.ConvCheckRecursiveSuffix))
	if clean == constant.ConvCheckRecursiveOnly {
		clean = constant.ConvCheckCurrentDir
	}
	info, err := os.Stat(clean)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		if strings.HasSuffix(clean, constant.ConvCheckGoExt) {
			return []goFilePath{{abs: clean, rel: slashPath(clean)}}, nil
		}
		return nil, nil
	}
	rootAbs, err := filepath.Abs(clean)
	if err != nil {
		return nil, err
	}
	base := pathBase(rootAbs)
	var paths []goFilePath
	err = filepath.WalkDir(rootAbs, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			name := entry.Name()
			if name == constant.ConvCheckSkipGitDir || name == constant.ConvCheckSkipWorktrees || name == constant.ConvCheckSkipVendor ||
				name == constant.ConvCheckSkipNodeModules || name == constant.ConvCheckSkipNextBuild {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(entry.Name(), constant.ConvCheckGoExt) {
			rel, err := filepath.Rel(base, path)
			if err != nil {
				return err
			}
			paths = append(paths, goFilePath{abs: path, rel: slashPath(rel)})
		}
		return nil
	})
	return paths, err
}

// pathBase 返回诊断路径的基准目录。
// 所有规则都按仓库根前缀（internal/、cmd/ …）判断，因此传入子目录时（如 ./internal/dto）
// 必须以工作目录为基准，否则文件会被归一成 dto/xxx.go，前缀规则集体失效。
// 目标目录不在工作目录内（如单元测试的临时模块）时回退到目标目录本身。
func pathBase(rootAbs string) string {
	workDir, err := os.Getwd()
	if err != nil {
		return rootAbs
	}
	rel, err := filepath.Rel(workDir, rootAbs)
	if err != nil {
		return rootAbs
	}
	rel = slashPath(rel)
	if rel == constant.ConvCheckParentDir || strings.HasPrefix(rel, constant.ConvCheckParentDirPrefix) {
		return rootAbs
	}
	return workDir
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

package fsutil

import (
	"errors"
)

var (
	// ErrFileExists 表示目标文件已存在
	ErrFileExists = errors.New("fsutil: target file already exists")
)

// ReplaceStrategy 定义了处理已存在目标文件的策略
type ReplaceStrategy int

const (
	// StrategyCreateNew 仅在文件不存在时创建。别名：StrategyNoOverwrite
	StrategyCreateNew ReplaceStrategy = iota
	// StrategyReplace 安全替换文件（如果已存在），否则创建新文件
	StrategyReplace
	// StrategyNoOverwrite 为了清晰提供的 StrategyCreateNew 的别名
	StrategyNoOverwrite = StrategyCreateNew
)

// PublishFile 安全地将临时文件移动到最终路径，使用指定策略。
// 它明确禁止在替换之前删除目标文件，以确保安全的原子替换或保留原始文件。
// 如果不能安全地替换，它将返回错误并保留旧文件和临时文件。
func PublishFile(tempPath, finalPath string, strategy ReplaceStrategy) error {
	switch strategy {
	case StrategyCreateNew:
		return publishNoOverwrite(tempPath, finalPath)
	case StrategyReplace:
		return publishReplace(tempPath, finalPath)
	default:
		return errors.New("fsutil: unknown replace strategy")
	}
}

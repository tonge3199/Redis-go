// Package logger 提供日志相关的文件操作工具函数
// 包含文件和目录检查、创建、权限验证等功能
package logger

import (
	"fmt"
	"os"
)

// checkNotExist 检查文件或目录是否存在
// 参数：
//
//	src: 要检查的文件或目录路径
//
// 返回值：
//
//	bool: true表示文件或目录不存在，false表示存在
func checkNotExist(src string) bool {
	_, err := os.Stat(src)
	return os.IsNotExist(err)
}

// checkPermission 检查文件或目录的权限
// 参数：
//
//	src: 要检查的文件或目录路径
//
// 返回值：
//
//	bool: true表示没有权限访问，false表示有权限访问
func checkPermission(src string) bool {
	_, err := os.Stat(src)
	return os.IsPermission(err)
}

// isNotExistMkDir 如果目录不存在则创建目录
// 参数：
//
//	src: 目录路径
//
// 返回值：
//
//	error: 创建过程中的错误，nil表示成功
//
// 功能：
//   - 检查目录是否存在
//   - 如果不存在则递归创建所有父目录
//   - 使用os.ModePerm权限（0755）
func isNotExistMkDir(src string) error {
	if checkNotExist(src) {
		return mkDir(src)
	}
	return nil
}

// mkDir 递归创建目录
// 参数：
//
//	src: 要创建的目录路径
//
// 返回值：
//
//	error: 创建过程中的错误，nil表示成功
//
// 说明：
//   - 使用os.MkdirAll递归创建所有父目录
//   - 设置权限为os.ModePerm（0755）
func mkDir(src string) error {
	err := os.MkdirAll(src, os.ModePerm)
	return err
}

// mustOpen 确保打开或创建日志文件
// 参数：
//
//	fileName: 日志文件名
//	dir: 日志文件所在目录路径
//
// 返回值：
//
//	*os.File: 文件句柄
//	error: 过程中的错误
//
// 功能流程：
//  1. 检查目录权限
//  2. 如果目录不存在则创建
//  3. 打开或创建文件（追加模式）
//  4. 设置文件权限为0644
//
// 错误处理：
//   - 权限不足：返回permission denied错误
//   - 目录创建失败：返回具体错误信息
//   - 文件打开失败：返回具体错误信息
func mustOpen(fileName, dir string) (*os.File, error) {
	if checkPermission(dir) {
		return nil, fmt.Errorf("permission denied dir: %s", dir)
	}

	if err := isNotExistMkDir(dir); err != nil {
		return nil, fmt.Errorf("error during make dir %s, err: %s", dir, err)
	}

	f, err := os.OpenFile(dir+string(os.PathSeparator)+fileName, os.O_APPEND|os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return nil, fmt.Errorf("error opening file: %s, err: %s", fileName, err)
	}

	return f, nil
}

/**
 * 构建 / 开发服务器启动前的必需文件检查。
 *
 * 替代 react-dev-utils/checkRequiredFiles：后者用已弃用的 fs.F_OK，
 * Node 22+ 启动时会打印 DEP0176 弃用警告。这里改用 fs.constants.F_OK。
 */
const fs = require('fs')
const path = require('path')
const chalk = require('react-dev-utils/chalk')

function checkRequiredFiles (files) {
  let currentFilePath
  try {
    files.forEach(filePath => {
      currentFilePath = filePath
      fs.accessSync(filePath, fs.constants.F_OK)
    })
    return true
  } catch (err) {
    const dirName = path.dirname(currentFilePath)
    const fileName = path.basename(currentFilePath)
    console.log(chalk.red('Could not find a required file.'))
    console.log(chalk.red('  Name: ') + chalk.cyan(fileName))
    console.log(chalk.red('  Searched in: ') + chalk.cyan(dirName))
    return false
  }
}

module.exports = checkRequiredFiles
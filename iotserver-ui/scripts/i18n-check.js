/**
 * 前端 i18n 词条覆盖检查：扫描 src 下所有 t('...') 调用，与 src/i18n/locales/en-US.json 对照。
 *
 *   node scripts/i18n-check.js
 *
 * 退出码非 0 表示「有文案没英文词条」——这类问题在页面上只会表现为「英文环境里冒出中文」，
 * 很难一眼看出来，所以放到 CI / 提交前跑。
 *
 * 约定见 src/i18n/index.ts：「词条 key 就是中文原文」，所以中文不需要资源文件。
 */
const fs = require('fs')
const path = require('path')

const parser = require('@babel/parser')

const SRC = path.resolve(__dirname, '../src')
const LOCALES = path.join(SRC, 'i18n/locales/en-US.json')

function listFiles (dir, out = []) {
  for (const name of fs.readdirSync(dir)) {
    if (name === 'i18n') continue // 词条文件自身不参与扫描
    const full = path.join(dir, name)
    const st = fs.statSync(full)
    if (st.isDirectory()) listFiles(full, out)
    else if (/\.(ts|tsx)$/.test(name) && !name.endsWith('.d.ts')) out.push(full)
  }
  return out
}

function walk (node, visit) {
  if (!node || typeof node !== 'object') return
  if (Array.isArray(node)) return node.forEach(item => walk(item, visit))
  if (typeof node.type === 'string') visit(node)
  for (const key of Object.keys(node)) {
    if (['loc', 'start', 'end', 'range'].includes(key)) continue
    const value = node[key]
    if (value && typeof value === 'object') walk(value, visit)
  }
}

const used = new Map()
for (const file of listFiles(SRC)) {
  const code = fs.readFileSync(file, 'utf8')
  let ast
  try {
    ast = parser.parse(code, {
      sourceType: 'module',
      plugins: ['typescript', 'jsx', 'classProperties', 'decorators-legacy', 'objectRestSpread', 'optionalChaining', 'nullishCoalescingOperator'],
    })
  } catch (error) {
    console.error('解析失败：' + path.relative(SRC, file) + ' ' + error.message)
    process.exitCode = 1
    continue
  }
  walk(ast.program, (node) => {
    if (node.type !== 'CallExpression') return
    const callee = node.callee
    if (!callee || callee.type !== 'Identifier' || callee.name !== 't') return
    const arg = node.arguments[0]
    if (!arg || arg.type !== 'StringLiteral') return
    const rel = path.relative(path.resolve(SRC, '..'), file) + ':' + node.loc.start.line
    if (!used.has(arg.value)) used.set(arg.value, [])
    used.get(arg.value).push(rel)
  })
}

const dict = JSON.parse(fs.readFileSync(LOCALES, 'utf8'))
const base = key => key.replace(/_(one|other|zero|few|many)$/, '')
const translated = new Set(Object.keys(dict).map(base))

const missing = [...used.keys()].filter(key => !translated.has(key) && /[一-鿿]/.test(key))
const unused = [...translated].filter(key => !used.has(key))

console.log('源码词条 ' + used.size + '，英文词条 ' + translated.size)
if (missing.length) {
  console.error('\n缺少英文词条 ' + missing.length + ' 条：')
  for (const key of missing) console.error('  ' + JSON.stringify(key) + '   ← ' + used.get(key).slice(0, 3).join(', '))
  process.exitCode = 1
}
if (unused.length) {
  // 注意：这里只是提示。后端下发的菜单名、看板里配置的标题/标签是运行时用 tt() 查词条的，
  // 静态扫描看不到，所以这些「用不到的」词条不一定真的废弃。
  console.warn('\n未在代码里直接引用的词条 ' + unused.length + ' 条（可能是 tt() 的数据文案，也可能已废弃）：')
  for (const key of unused.slice(0, 40)) console.warn('  ' + JSON.stringify(key))
  if (unused.length > 40) console.warn('  ...还有 ' + (unused.length - 40) + ' 条')
}
if (!missing.length) console.log('✅ 没有缺失的英文词条')
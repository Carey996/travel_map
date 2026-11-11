#!/usr/bin/env node
/**
 * 前端 i18n 巡检脚本（无第三方依赖，只用 Node 内置模块）。
 *
 * 职责：
 *   1. 递归扫描 web/src 下所有 .vue / .ts，提取 `t('...')` 与 `$t('...')` 的键名；
 *   2. 与 locales/zh.json、en.json 做交叉校验：
 *        - 字面量键必须两种语言都存在；
 *        - 动态拼接键（如 `t('place.kind.' + kind)`）按**前缀**处理，
 *          只要存在以该前缀开头的词条即视为已覆盖；
 *        - zh / en 的键集合必须完全一致（缺 / 多都报）；
 *   3. 顺手守住项目硬约束：词条值里出现 `{{...}}` 双大括号即报错
 *      （vue-i18n 单大括号插值，双大括号会在运行时抛错，整棵组件树渲染失败）；
 *   4. 全部通过时打印统计（used / zh / en 数量）；有缺失时打印清单并以非 0 退出。
 *
 * 用法：在 web/ 目录下 `node scripts/check-i18n.mjs`（或 `npm run check:i18n`）。
 */

import { readFileSync, readdirSync } from 'node:fs';
import { dirname, extname, join, relative, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const webRoot = resolve(dirname(fileURLToPath(import.meta.url)), '..');
const srcDir = join(webRoot, 'src');
const localesDir = join(srcDir, 'i18n', 'locales');

const LANGS = ['zh', 'en'];
const SCAN_EXTS = new Set(['.vue', '.ts']);
const SKIP_DIRS = new Set(['node_modules', 'dist', 'coverage', '.git']);

/** 需要扫描的源文件（相对 web/ 的 POSIX 路径，便于稳定输出）。 */
function collectSourceFiles(dir) {
  const files = [];
  for (const entry of readdirSync(dir, { withFileTypes: true })) {
    const full = join(dir, entry.name);
    if (entry.isDirectory()) {
      if (SKIP_DIRS.has(entry.name)) continue;
      files.push(...collectSourceFiles(full));
    } else if (entry.isFile() && SCAN_EXTS.has(extname(entry.name))) {
      files.push(full);
    }
  }
  return files.sort();
}

/**
 * 把注释替换成空格：保留字符长度与换行，因此行号不变，
 * 同时避免注释里示例性的 t('key') 被误统计。字符串字面量原样保留。
 */
function stripComments(code) {
  const out = code.split('');
  const n = code.length;
  let i = 0;
  let state = 'code'; // code | line | block | html
  while (i < n) {
    const ch = code[i];
    const next = code[i + 1];
    if (state === 'code') {
      if (ch === '/' && next === '/') {
        out[i] = ' ';
        out[i + 1] = ' ';
        state = 'line';
        i += 2;
        continue;
      }
      if (ch === '/' && next === '*') {
        out[i] = ' ';
        out[i + 1] = ' ';
        state = 'block';
        i += 2;
        continue;
      }
      if (ch === '<' && code.startsWith('<!--', i)) {
        for (let k = 0; k < 4; k += 1) out[i + k] = ' ';
        state = 'html';
        i += 4;
        continue;
      }
      if (ch === '"' || ch === "'" || ch === '`') {
        // 跳过整个字符串字面量（含转义），不修改内容
        const quote = ch;
        i += 1;
        while (i < n) {
          if (code[i] === '\\') {
            i += 2;
            continue;
          }
          if (code[i] === quote) {
            i += 1;
            break;
          }
          i += 1;
        }
        continue;
      }
      i += 1;
      continue;
    }
    if (state === 'line') {
      if (ch === '\n') {
        state = 'code';
        i += 1;
        continue;
      }
      out[i] = ' ';
      i += 1;
      continue;
    }
    if (state === 'block') {
      if (ch === '*' && next === '/') {
        out[i] = ' ';
        out[i + 1] = ' ';
        state = 'code';
        i += 2;
        continue;
      }
      if (ch !== '\n') out[i] = ' ';
      i += 1;
      continue;
    }
    // html 注释
    if (ch === '-' && code.startsWith('-->', i)) {
      for (let k = 0; k < 3; k += 1) out[i + k] = ' ';
      state = 'code';
      i += 3;
      continue;
    }
    if (ch !== '\n') out[i] = ' ';
    i += 1;
  }
  return out.join('');
}

/** 解析从 start（引号位置）开始的字符串字面量。 */
function parseStringLiteral(code, start) {
  const quote = code[start];
  let i = start + 1;
  let value = '';
  while (i < code.length) {
    const ch = code[i];
    if (ch === '\\') {
      value += code[i + 1] ?? '';
      i += 2;
      continue;
    }
    if (ch === quote) return { value, end: i + 1, interpolated: false };
    // 模板字符串里出现 ${...}：静态部分到此为止，整体按前缀处理
    if (quote === '`' && ch === '$' && code[i + 1] === '{') {
      return { value, end: i, interpolated: true };
    }
    value += ch;
    i += 1;
  }
  return null; // 未闭合，交给上层忽略
}

const CALL_RE = /(?<![\w$])t\s*\(/g;
const QUOTES = new Set(["'", '"', '`']);
const WHITESPACE = /[ \t\r\n]/;

/**
 * 提取一个文件里的 t() / $t() 调用。
 * 返回 [{ kind: 'literal' | 'prefix' | 'dynamic', key, line, snippet }]
 */
function extractCalls(code) {
  const calls = [];
  CALL_RE.lastIndex = 0;
  let match;
  while ((match = CALL_RE.exec(code)) !== null) {
    let i = match.index + match[0].length;
    while (i < code.length && WHITESPACE.test(code[i])) i += 1;
    if (i >= code.length) break;

    const line = code.slice(0, match.index).split('\n').length;
    const snippet = code.slice(match.index, match.index + 80).split('\n')[0].trim();
    const ch = code[i];

    if (!QUOTES.has(ch)) {
      // 参数是变量 / 函数调用等，无法静态判定
      calls.push({ kind: 'dynamic', key: null, line, snippet });
      continue;
    }

    const literal = parseStringLiteral(code, i);
    if (!literal) continue;

    let after = literal.end;
    while (after < code.length && WHITESPACE.test(code[after])) after += 1;

    if (literal.interpolated) {
      // `photo.${kind}` 这类模板字符串：静态部分即前缀
      calls.push({ kind: 'prefix', key: literal.value, line, snippet });
    } else if (code[after] === '+') {
      // t('place.kind.' + kind) 这类拼接：字面量即前缀
      calls.push({ kind: 'prefix', key: literal.value, line, snippet });
    } else {
      calls.push({ kind: 'literal', key: literal.value, line, snippet });
    }
  }
  return calls;
}

/** 把嵌套词条展平成 { 'a.b.c': value }。 */
function flattenKeys(tree, prefix = '', out = new Map()) {
  for (const [key, value] of Object.entries(tree)) {
    const full = prefix ? `${prefix}.${key}` : key;
    if (value !== null && typeof value === 'object' && !Array.isArray(value)) {
      flattenKeys(value, full, out);
    } else {
      out.set(full, value);
    }
  }
  return out;
}

function loadLocale(lang) {
  const file = join(localesDir, `${lang}.json`);
  let raw;
  try {
    raw = readFileSync(file, 'utf8');
  } catch (error) {
    fail(`无法读取 ${relative(webRoot, file)}：${error.message}`);
    return null;
  }
  try {
    return flattenKeys(JSON.parse(raw));
  } catch (error) {
    fail(`${relative(webRoot, file)} 不是合法 JSON：${error.message}`);
    return null;
  }
}

const errors = [];
const warnings = [];

function fail(message) {
  errors.push(message);
}

function main() {
  // ---------- 1. 读词条 ----------
  const locales = {};
  for (const lang of LANGS) locales[lang] = loadLocale(lang);
  if (LANGS.some((lang) => !locales[lang])) return report();

  const zh = locales.zh;
  const en = locales.en;

  // ---------- 2. 扫描源码 ----------
  const files = collectSourceFiles(srcDir);
  const literalKeys = new Map(); // key -> first location
  const prefixKeys = new Map(); // prefix -> first location
  const dynamicCalls = []; // { file, line, snippet }

  for (const file of files) {
    const rel = relative(webRoot, file).split('\\').join('/');
    const code = stripComments(readFileSync(file, 'utf8'));
    const calls = extractCalls(code);
    for (const call of calls) {
      if (call.kind === 'dynamic') {
        dynamicCalls.push({ file: rel, line: call.line, snippet: call.snippet });
        continue;
      }
      const bucket = call.kind === 'literal' ? literalKeys : prefixKeys;
      if (!bucket.has(call.key)) bucket.set(call.key, `${rel}:${call.line}`);
    }
  }

  // ---------- 3. 引用键校验 ----------
  for (const [key, where] of literalKeys) {
    if (!zh.has(key)) errors.push(`[缺失] zh.json 缺少词条 "${key}"（被 ${where} 引用）`);
    if (!en.has(key)) errors.push(`[缺失] en.json 缺少词条 "${key}"（被 ${where} 引用）`);
  }

  for (const [prefix, where] of prefixKeys) {
    const zhHit = [...zh.keys()].some((key) => key.startsWith(prefix));
    const enHit = [...en.keys()].some((key) => key.startsWith(prefix));
    if (!zhHit) errors.push(`[动态前缀] zh.json 没有以 "${prefix}" 开头的词条（来自 ${where}）`);
    if (!enHit) errors.push(`[动态前缀] en.json 没有以 "${prefix}" 开头的词条（来自 ${where}）`);
  }

  // ---------- 4. zh / en 键集一致性 ----------
  const zhOnly = [...zh.keys()].filter((key) => !en.has(key)).sort();
  const enOnly = [...en.keys()].filter((key) => !zh.has(key)).sort();
  for (const key of zhOnly) errors.push(`[不对称] zh.json 有而 en.json 缺："${key}"`);
  for (const key of enOnly) errors.push(`[不对称] en.json 有而 zh.json 缺："${key}"`);

  // ---------- 5. 单大括号插值约束（双大括号会运行时抛错） ----------
  for (const lang of LANGS) {
    for (const [key, value] of locales[lang]) {
      if (typeof value === 'string' && value.includes('{{')) {
        errors.push(`[双大括号] ${lang}.json 的 "${key}" 含 {{...}}，vue-i18n 要求单大括号插值`);
      }
    }
  }

  // ---------- 6. 占位符一致性（只告警，不改语义） ----------
  const placeholderRe = /\{([a-zA-Z_$][\w$]*)\}/g;
  const placeholders = (value) =>
    typeof value === 'string'
      ? [...new Set([...value.matchAll(placeholderRe)].map((m) => m[1]))].sort().join(',')
      : '';
  for (const key of zh.keys()) {
    if (!en.has(key)) continue;
    const z = placeholders(zh.get(key));
    const e = placeholders(en.get(key));
    if (z !== e) {
      warnings.push(`[占位符] "${key}" 的插值参数不一致：zh=[${z}] en=[${e}]`);
    }
  }

  if (dynamicCalls.length) {
    for (const call of dynamicCalls) {
      warnings.push(`[动态] 无法静态判定 ${call.file}:${call.line} 的 t() 参数：${call.snippet}`);
    }
  }

  // ---------- 7. 输出 ----------
  const usedLiteral = literalKeys.size;
  const usedPrefix = prefixKeys.size;
  const usedTotal = usedLiteral + usedPrefix;
  const scanned = files.length;

  if (errors.length) {
    console.error(`i18n 巡检未通过：发现 ${errors.length} 个问题\n`);
    for (const message of errors) console.error(`  ✗ ${message}`);
    if (warnings.length) {
      console.error(`\n另 ${warnings.length} 条提示：`);
      for (const message of warnings) console.error(`  ! ${message}`);
    }
    console.error(
      `\n统计：扫描 ${scanned} 个文件，引用键 ${usedTotal}（字面量 ${usedLiteral} / 动态前缀 ${usedPrefix}），` +
        `zh ${zh.size} 条 / en ${en.size} 条`,
    );
    report();
    return;
  }

  console.log(`i18n 巡检通过`);
  console.log(`  扫描文件：${scanned} 个（web/src 下 .vue / .ts）`);
  console.log(`  引用键：${usedTotal}（字面量 ${usedLiteral} / 动态前缀 ${usedPrefix}）`);
  console.log(`  词条数：zh ${zh.size} / en ${en.size}（键集合一致）`);
  console.log(`  引用覆盖：${usedTotal} 个引用键全部在 zh / en 中存在`);
  if (warnings.length) {
    console.log(`  提示：${warnings.length} 条`);
    for (const message of warnings) console.log(`    ! ${message}`);
  }
  report();
}

function report() {
  if (errors.length) process.exitCode = 1;
}

main();

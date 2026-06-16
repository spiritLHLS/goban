#!/usr/bin/env node

const fs = require('fs')

const routeSource = fs.readFileSync('server/internal/routes/routes.go', 'utf8')
const apiSource = fs.readFileSync('web/src/api/index.js', 'utf8')
const docsSource = fs.readFileSync('server/internal/docs/openapi.go', 'utf8')

const groups = {
  router: '',
  api: '/api',
  auth: '/api'
}
const backendRoutes = new Set()

for (const rawLine of routeSource.split(/\r?\n/)) {
  const line = rawLine.trim()

  const groupMatch = line.match(/^(\w+)\s*:=\s*(\w+)\.Group\("([^"]*)"\)/)
  if (groupMatch) {
    const [, name, parent, path] = groupMatch
    groups[name] = joinPath(groups[parent] || '', path)
    continue
  }

  const routeMatch = line.match(/^(\w+)\.(GET|POST|PUT|DELETE)\("([^"]*)"/)
  if (routeMatch) {
    const [, group, method, path] = routeMatch
    const fullPath = normalizeRoute(joinPath(groups[group] || '', path))
    if (fullPath === '/api' || fullPath.startsWith('/api/')) {
      backendRoutes.add(`${method} ${fullPath}`)
    }
  }
}

const frontendRoutes = new Set()
const callRegex = /request\.(get|post|put|delete)\((`[^`]+`|'[^']+'|"[^"]+")/g
let match
while ((match = callRegex.exec(apiSource)) !== null) {
  const method = match[1].toUpperCase()
  const path = normalizeTemplate(match[2].slice(1, -1))
  frontendRoutes.add(`${method} ${normalizeRoute(joinPath('/api', path))}`)
}

const missing = [...frontendRoutes].filter(route => !backendRoutes.has(route)).sort()
if (missing.length > 0) {
  console.error('Frontend API calls missing backend routes:')
  for (const route of missing) {
    console.error(`  - ${route}`)
  }
  process.exit(1)
}

const specMatch = docsSource.match(/const OpenAPIJSON = `([\s\S]*?)`/)
if (!specMatch) {
  console.error('OpenAPIJSON const not found')
  process.exit(1)
}

const spec = JSON.parse(specMatch[1])
const documentedRoutes = new Set()
for (const [path, operations] of Object.entries(spec.paths || {})) {
  for (const method of Object.keys(operations)) {
    documentedRoutes.add(`${method.toUpperCase()} ${normalizeRoute(path)}`)
  }
}

const undocumented = [...backendRoutes].filter(route => !documentedRoutes.has(route)).sort()
if (undocumented.length > 0) {
  console.error('Backend routes missing from OpenAPI spec:')
  for (const route of undocumented) {
    console.error(`  - ${route}`)
  }
  process.exit(1)
}

console.log(`API route check passed: ${frontendRoutes.size} frontend calls, ${backendRoutes.size} backend routes`)

function joinPath(prefix, path) {
  const left = prefix.endsWith('/') ? prefix.slice(0, -1) : prefix
  const right = path.startsWith('/') ? path : `/${path}`
  return `${left}${right}`.replace(/\/+/g, '/')
}

function normalizeRoute(path) {
  return path
    .replace(/\/+/g, '/')
    .replace(/\/:([^/]+)/g, '/{$1}')
    .replace(/\/$/, '') || '/'
}

function normalizeTemplate(path) {
  return path
    .replace(/\$\{[^}]+\}/g, ':id')
    .replace(/`/g, '')
}

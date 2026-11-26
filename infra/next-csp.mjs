import { createHash } from "node:crypto"
import { readFileSync, readdirSync, writeFileSync } from "node:fs"
import { join } from "node:path"

const [directory, destination] = process.argv.slice(2)
const pages = readdirSync(directory, { recursive: true }).filter((path) => path.endsWith(".html"))
if (!pages.includes("index.html")) throw new Error("Next.js export is missing its home page")

const entries = []
for (const page of pages.sort()) {
  const html = readFileSync(join(directory, page), "utf8")
  const hashes = [...html.matchAll(/<script\b([^>]*)>([\s\S]*?)<\/script\s*>/gi)]
    .filter(([, attributes, body]) => body && !/\ssrc\s*=/i.test(attributes))
    .map(([, , body]) => "'sha256-" + createHash("sha256").update(body).digest("base64") + "'")
  const policy = [...new Set(hashes)].join(" ")
  const routes = ["/" + page]
  if (page.endsWith("index.html")) routes.push("/" + page.slice(0, -"index.html".length))
  for (const route of routes) {
    if (!/^\/[a-z0-9_./-]*$/i.test(route)) throw new Error("Unsupported route in static export: " + route)
    entries.push(`    "${route}" "${policy}";`)
  }
}

writeFileSync(destination, `map $uri $next_script_hashes {\n    default "";\n${entries.join("\n")}\n}\n`)

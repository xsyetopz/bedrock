# Bun APIs and Node.js Modules

Use the Bun API where Bun has one, and a `node:` module where it has none. Import Node.js modules with the `node:` prefix; `unicorn/prefer-node-protocol` checks it. For YAML, TOML, JSONC, and JSON5, use `Bun.YAML`, `Bun.TOML`, `Bun.JSONC`, and `Bun.JSON5`.

The table tracks how close the repo is to banning `node:` imports outright. "Runs" and "Gaps" come from Bun's [compatibility page](https://bun.com/docs/runtime/nodejs-compat), which targets Node.js v26. "Replacement" names the Bun or web API that does the same job, checked against Bun 1.4.4. "Covered" says whether that replacement covers what code uses the module for.

- ✅ In "Runs", Bun implements the module. In "Covered", the import can be banned today.
- ⚠️ Partly. In "Covered", the cell names the uses that still need the module.
- ❌ In "Runs", Bun lacks the module. In "Covered", Bun has no replacement.
- 🗑️ Deprecated in Node.js. Remove the code that uses it.

## Every Module

| Module | Runs | Gaps in Bun's `node:` version | Replacement | Covered |
| --- | --- | --- | --- | --- |
| `assert` | ✅ | Legacy `deepEqual` uses `Bun.deepEquals` rules; function and printf-style messages aren't formatted | `expect` from `bun:test`, in tests only | ❌ |
| `async_hooks` | ⚠️ | `createHook`, `executionAsyncId`, `triggerAsyncId`, `executionAsyncResource` are stubs; `AsyncLocalStorage` doesn't reach `MessagePort`, `BroadcastChannel`, or `Worker` | None | ❌ `AsyncLocalStorage` |
| `buffer` | ✅ | 4 GiB per `Buffer` | `Uint8Array`, `TextEncoder`, `TextDecoder` | ⚠️ the `Buffer` global is itself Node.js |
| `child_process` | ⚠️ | IPC can't send `http` server sockets; `advanced` serialization only between Bun processes; no `subprocess.channel.ref()` or `unref()` | `Bun.spawn`, `Bun.$` | ✅ |
| `cluster` | ⚠️ | Load balancing only on Linux, through `SO_REUSEPORT` | `Bun.serve({ reusePort })` with `Bun.spawn` | ⚠️ no primary/worker API |
| `console` | ✅ | Writes to file descriptors, so replacing `process.stdout.write` doesn't capture it; layout differs from `util.inspect` | The global `console` | ✅ |
| `crypto` | ⚠️ | No `encapsulate` or `decapsulate`; `setEngine()` throws; `setFips()` does nothing; BoringSSL lacks some key types, curves, and ciphers | `crypto.subtle`, `crypto.randomUUID`, `Bun.CryptoHasher`, `Bun.hash`, `Bun.password` | ⚠️ `KeyObject`, `createCipheriv`, and streaming ciphers |
| `dgram` | ✅ | `addMembership()` needs `bind()` first | `Bun.udpSocket` | ✅ |
| `diagnostics_channel` | ⚠️ | No `boundedChannel()` and several built-in channels | None | ❌ |
| `dns` | ✅ | No `resolveTlsa`; `maxTimeout` is ignored | `Bun.dns`: `lookup`, every `resolve*`, `reverse`, `lookupService`, `prefetch` | ✅ |
| `domain` | ⚠️ | No `members`; errors from async callbacks skip the domain | None | 🗑️ |
| `events` | ✅ | `EventEmitterAsyncResource` always has `asyncId` `0` | `EventTarget` | ⚠️ `EventEmitter`, `once()`, and `on()` |
| `fs` | ✅ | `Stats` has no Temporal getters | `Bun.file` (`text`, `stream`, `stat`, `exists`, `delete`), `Bun.write`, `Bun.Glob` | ⚠️ `mkdir`, `readdir`, `rename`, `rm`, and `watch` |
| `http` | ✅ | `http.Server` doesn't extend `net.Server`; some `listen()` and `keepAlive` options are ignored | `Bun.serve`, `fetch` | ✅ |
| `http2` | ✅ | `maxDeflateDynamicTableSize`, `peerMaxConcurrentStreams`, and stream reset options are ignored | `Bun.serve({ http2 })` and `fetch` with `protocol: "http2"`, both experimental | ⚠️ until they're stable |
| `https` | ⚠️ | No `getPeerCertificate()` or `getCipher()` on request sockets; no `setSecureContext()`, `addContext()`, `SNICallback`, or `handshakeTimeout` | `Bun.serve({ tls })`, `fetch` | ✅ |
| `inspector` | ⚠️ | No `Network`; `Runtime.evaluate` and `HeapProfiler` aren't implemented | `bun --inspect`, `startRemoteDebugger` from `bun:jsc` | ⚠️ no in-process sessions |
| `module` | ⚠️ | No `Module#load()`, `registerHooks`, `findPackageJSON`, and others; `register` does nothing; `findSourceMap` returns `undefined` | `Bun.plugin`, `Bun.resolveSync`, `import.meta.resolve` | ⚠️ `createRequire` |
| `net` | ✅ | Can't read from an existing fd; `listen(handle)` takes only `{ fd }`; no `BlockList` `toJSON()` or `fromJSON()` | `Bun.listen`, `Bun.connect` | ⚠️ `isIP` and `BlockList` |
| `os` | ✅ | `userInfo()` reads environment variables; `machine()` returns `arm64` on Linux arm64 | None | ❌ |
| `path` | ✅ | `matchesGlob()` follows `Bun.Glob` rules; `path.win32` differs for some device paths and reserved names | None | ❌ |
| `perf_hooks` | ⚠️ | `eventLoopUtilization()` returns zeros; no `gc`, `dns`, or `resource` entries | `performance`, `Bun.nanoseconds` | ⚠️ histograms and `monitorEventLoopDelay` |
| `process` | ⚠️ | `process.binding` is partial; setting `process.title` does nothing on macOS and Linux | `Bun.env`, `Bun.argv`, `Bun.main`, `Bun.stdin`, `Bun.stdout`, `Bun.stderr` | ⚠️ `exit`, `cwd`, signals, and the `process` global |
| `punycode` | ✅ | None listed | None | 🗑️ |
| `querystring` | ✅ | None listed | `URLSearchParams` | ✅ |
| `quic` | ✅ | Experimental; importing it warns | `Bun.serve({ http3 })` and `fetch` with `protocol: "http3"`, both experimental | ⚠️ no raw QUIC streams |
| `readline` | ✅ | None listed | `for await (const line of console)` | ⚠️ stdin lines only |
| `repl` | ⚠️ | No result previews; tab completion skips `let`, `const`, and `class` | `bun repl` | ❌ nothing to embed |
| `sea` | ❌ | Not implemented | `bun build --compile` | ✅ |
| `sqlite` | ✅ | `backup()` blocks the event loop; non-UTF-8 `Buffer` paths are rejected; `loadExtension()` needs a full SQLite build | `bun:sqlite` | ✅ |
| `stream` | ✅ | `isReadable`, `isWritable`, `isErrored`, and `isDisturbed` handle only Node.js streams | `ReadableStream`, `WritableStream`, `TransformStream` | ⚠️ libraries that take Node.js streams |
| `string_decoder` | ✅ | `end()` takes no string | `TextDecoder` with `{ stream: true }`, `TextDecoderStream` | ✅ |
| `sys` | ✅ | Same as `util` | Same as `util` | 🗑️ |
| `test` | ⚠️ | Most `run()` options throw; no snapshots, `mock.module()`, or coverage; `test.only()` doesn't filter | `bun:test` | ✅ |
| `timers` | ✅ | None listed | Global timers, `Bun.sleep` | ✅ |
| `tls` | ⚠️ | No `pskCallback`, OCSP stapling, or server session events; `renegotiate()` always fails | `Bun.listen` and `Bun.connect` with `tls` | ⚠️ those missing features |
| `trace_events` | ✅ | Some categories record less than in Node.js | None | ❌ |
| `tty` | ✅ | Streams extend `fs` streams; a non-TTY fd gives `isTTY: false` instead of throwing | `Bun.Terminal`, for pseudo-terminals | ❌ `isatty` and window size |
| `url` | ✅ | None listed | `URL`, `URLSearchParams`, `URLPattern`, `Bun.fileURLToPath`, `Bun.pathToFileURL` | ⚠️ `domainToASCII` and legacy `parse` and `format` |
| `util` | ✅ | No `diff`, `transferableAbortSignal`, or `transferableAbortController`; `debuglog()` ignores its callback | `Bun.inspect`, `Bun.deepEquals`, `Bun.sleep`, `Bun.stripANSI`, `Bun.color` | ⚠️ `parseArgs`, `promisify`, and `styleText` |
| `v8` | ⚠️ | No `queryObjects`, CPU or heap profiling, `Serializer` or `Deserializer`, or coverage functions | `bun:jsc`: `serialize`, `deserialize`, `heapStats`, `memoryUsage`, `startSamplingProfiler`, `generateHeapSnapshotForDebugging` | ✅ |
| `vm` | ✅ | An `importModuleDynamically` promise resolves to the module object; `measureMemory()` reports the whole heap | `ShadowRealm` | ⚠️ contexts and `Script` |
| `wasi` | ⚠️ | No `getImportObject()`, `initialize()`, or `sock_accept` | None | ❌ |
| `worker_threads` | ⚠️ | Ignores `resourceLimits` and `trackUnmanagedFds`; no `moveMessagePortToContext` or `locks` | `Worker`, `Worker.data`, `Bun.isMainThread`, `MessageChannel`, `BroadcastChannel` | ✅ |
| `zlib` | ✅ | None listed | `Bun.gzipSync`, `Bun.deflateSync`, `Bun.zstdCompress`, `CompressionStream` | ✅ |

## What Blocks a Full Ban

A blanket ban on `node:` imports waits on the ❌ rows that ordinary code uses: `path`, `os`, `assert`, and `AsyncLocalStorage` from `async_hooks`. After those, the ⚠️ rows most code hits are directory operations in `fs`, `EventEmitter` in `events`, and `parseArgs` in `util`. The `Buffer` and `process` globals are Node.js APIs too, and an import ban doesn't touch them.

Lint already bans `node:child_process`, `node:sqlite`, and `node:test`, plus the npm packages Bun replaces. [lint-rules.md](lint-rules.md) has that list. Move a module to that list when its "Covered" cell reads ✅.

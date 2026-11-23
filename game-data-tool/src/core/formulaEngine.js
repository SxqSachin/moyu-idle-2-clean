let seq = 0
let worker = null
const pending = new Map()

function createWorker() {
  worker = new Worker(new URL("../workers/formulaWorker.js", import.meta.url), { type: "module" })
  worker.onmessage = (event) => {
    const { id, ok, value, error } = event.data
    const task = pending.get(id)
    if (!task) return
    pending.delete(id)
    if (ok) task.resolve(value)
    else task.reject(new Error(error || "公式执行失败"))
  }
}

function ensureWorker() {
  if (!worker) createWorker()
}

function resetWorker() {
  if (worker) worker.terminate()
  worker = null
  createWorker()
}

export async function runFormula(expression, context, timeoutMs = 120) {
  ensureWorker()
  const id = ++seq
  let payloadContext
  try {
    payloadContext = typeof structuredClone === "function" ? structuredClone(context || {}) : JSON.parse(JSON.stringify(context || {}))
  } catch {
    payloadContext = JSON.parse(JSON.stringify(context || {}))
  }
  return new Promise((resolve, reject) => {
    const timer = setTimeout(() => {
      pending.delete(id)
      resetWorker()
      reject(new Error(`公式执行超时 (${timeoutMs}ms)`))
    }, timeoutMs)
    pending.set(id, {
      resolve: (value) => {
        clearTimeout(timer)
        resolve(value)
      },
      reject: (error) => {
        clearTimeout(timer)
        reject(error)
      }
    })
    worker.postMessage({ id, expression, context: payloadContext })
  })
}

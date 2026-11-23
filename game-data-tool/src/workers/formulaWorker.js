self.onmessage = async (event) => {
  const { id, expression, context } = event.data
  try {
    const fn = new Function(
      "ctx",
      `
      const Math = globalThis.Math;
      const Number = globalThis.Number;
      const String = globalThis.String;
      const Boolean = globalThis.Boolean;
      const Date = undefined;
      const Function = undefined;
      const fetch = undefined;
      with (ctx) {
        return (${expression});
      }
    `
    )
    const value = fn(context || {})
    self.postMessage({ id, ok: true, value })
  } catch (error) {
    self.postMessage({ id, ok: false, error: error instanceof Error ? error.message : String(error) })
  }
}

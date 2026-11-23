export function validateCatteryFurniture(rows, items, actions = [], mainTypes = [], subTypes = []) {
  if (!Array.isArray(rows) || rows.length > 32) throw new Error('Invalid furniture catalog')
  const itemIds = new Set(items.map(x => x.id))
  const ids = new Set()
  const exact = (obj, fields) => {
    if (!obj || typeof obj !== 'object' || Array.isArray(obj) || Object.keys(obj).some(k => !fields.includes(k))) throw new Error('Invalid furniture fields')
  }
  for (const d of rows || []) {
    exact(d, ['id','version','nameKey','iconId','installCosts','refillCosts','capacitySeconds','refillSeconds','rules','levels','service'])
    if (!/^[a-z][a-z0-9_]{0,79}$/.test(d.id) || ids.has(d.id) || typeof d.version !== 'string' || !d.version || d.version.length > 80 || typeof d.nameKey !== 'string' || !d.nameKey || !/^item\.[a-z][a-z0-9_]*$/.test(d.iconId)) throw new Error('Invalid furniture identity')
    ids.add(d.id)
    const permanent = d.service === 'permanent'
    if (d.service !== undefined && !['permanent','prepaid'].includes(d.service)) throw new Error('Invalid furniture service')
    if (permanent) {
      if ((d.capacitySeconds ?? 0) !== 0 || (d.refillSeconds ?? 0) !== 0 || (d.refillCosts !== undefined && (!d.refillCosts || Array.isArray(d.refillCosts) || Object.keys(d.refillCosts).length))) throw new Error('Permanent furniture cannot require maintenance')
    } else if (!Number.isSafeInteger(d.capacitySeconds) || d.capacitySeconds <= 0 || d.capacitySeconds > 604800 || !Number.isSafeInteger(d.refillSeconds) || d.refillSeconds <= 0 || d.refillSeconds > d.capacitySeconds) throw new Error('Invalid furniture service duration')
    for (const costs of permanent ? [d.installCosts] : [d.installCosts, d.refillCosts]) {
      if (!costs || Array.isArray(costs) || Object.keys(costs).length < 1 || Object.keys(costs).length > 24) throw new Error('Invalid furniture costs')
      for (const [id,n] of Object.entries(costs)) if (!itemIds.has(id) || !Number.isSafeInteger(n) || n <= 0 || n > 1000000) throw new Error('Invalid furniture material')
    }
    if (d.levels !== undefined) {
      if (!Array.isArray(d.levels) || d.levels.length > 9) throw new Error('Invalid furniture levels')
      let refill = { ...d.refillCosts }
      for (const [i, stage] of d.levels.entries()) {
        exact(stage, ['level', 'upgradeCosts', 'refillCostAdd', 'rules'])
        if (stage.level !== i + 2 || (!permanent && (!stage.refillCostAdd || !Object.keys(stage.refillCostAdd).length)) || (permanent && stage.refillCostAdd !== undefined && (!stage.refillCostAdd || Array.isArray(stage.refillCostAdd) || Object.keys(stage.refillCostAdd).length))) throw new Error('Invalid furniture progression')
        for (const [id, n] of Object.entries(stage.refillCostAdd || {})) {
          if (!Number.isSafeInteger(n) || n <= 0) throw new Error('Maintenance cannot remove a lower-tier material')
          refill[id] = (refill[id] || 0) + n
        }
        validateCatteryFurniture([{ ...d, levels: undefined, installCosts: stage.upgradeCosts, refillCosts: refill, rules: stage.rules }], items, actions, mainTypes, subTypes)
      }
    }
    if (!Array.isArray(d.rules) || !d.rules.length || d.rules.length > 8) throw new Error('Invalid furniture rules')
    const rules = new Set()
    for (const r of d.rules) {
      exact(r,['kind','domain','rateBps','selector'])
      const key = `${r.kind}:${r.domain}:${JSON.stringify(r.selector || {})}`
      if (rules.has(key)) throw new Error('Duplicate furniture rule')
      rules.add(key)
      const efficiency = r.kind === 'action_efficiency_add' && r.domain === 'action_start'
      if (!Number.isSafeInteger(r.rateBps) || r.rateBps < 0 || r.rateBps > (efficiency ? 20000 : 10000)) throw new Error('Invalid furniture rate')
      if (!(r.kind === 'owner_experience' && ['life_action','fishing_cast'].includes(r.domain)) && !(r.kind === 'kitty_extra_loot' && r.domain === 'battle_reward') && !efficiency) throw new Error('Unknown furniture rule')
      if (efficiency && (!r.selector?.mainTypes?.length || r.selector.mainTypes.some(x => !['collecting','mining','farming_animal','crafting','sewing','forging','alchemy','cooking'].includes(x)))) throw new Error('Invalid work efficiency scope')
      if (r.selector) {
        exact(r.selector, ['actionIds', 'mainTypes', 'subTypes'])
        if (r.kind === 'kitty_extra_loot' && Object.keys(r.selector).length) throw new Error('Battle reward selectors are unsupported')
        for (const [field, references] of [['actionIds', actions], ['mainTypes', mainTypes], ['subTypes', subTypes]]) {
          const values = r.selector[field]
          if (values === undefined) continue
          const allowed = new Set(references.map(x => x.id))
          if (!Array.isArray(values) || values.length > 64 || values.some(x => typeof x !== 'string' || !allowed.has(x)) || new Set(values).size !== values.length) throw new Error('Invalid furniture selector reference')
        }
      }
    }
  }
}

import type { WorldOptionField, WorldOptionGroup } from '@/types/api'

export type PlaystyleId = 'relaxed' | 'endless' | 'survival' | 'wilderness' | 'lightsout'

export interface PlaystyleDefinition {
  id: PlaystyleId
  presetId: string
  gameMode: 'relaxed' | 'endless' | 'survival' | 'wilderness'
  nameZh: string
  nameEn: string
  shortDescZh: string
  shortDescEn: string
  detailDescZh: string
  detailDescEn: string
  tagsZh: string[]
  tagsEn: string[]
  accentClass: string
  badgeClass: string
}

export const PLAYSTYLES: PlaystyleDefinition[] = [
  {
    id: 'relaxed',
    presetId: 'relaxed',
    gameMode: 'relaxed',
    nameZh: '轻松',
    nameEn: 'Relaxed',
    shortDescZh: '更轻松的游戏方式，更少受到来自世界的威胁。',
    shortDescEn: 'A more relaxed playstyle with fewer threats from the world.',
    detailDescZh:
      '更轻松的游戏方式，更少受到来自世界的威胁。饥饿、寒冷、过热和黑暗将不会杀死冒险家。降低冒险家受到的伤害。永远可以在绚丽之门复活。',
    detailDescEn:
      'A more relaxed playstyle with fewer threats from the world. Hunger, freezing, overheating, and darkness will not kill survivors. Damage taken is reduced, and you can always revive at the Florid Postern.',
    tagsZh: ['环境伤害非致命', '减伤保护', '绚丽之门复活'],
    tagsEn: ['Non-lethal hazards', 'Less damage', 'Portal revive'],
    accentClass: 'bg-emerald-500/12 border-emerald-500/40',
    badgeClass: 'bg-emerald-500/12 text-emerald-700 dark:text-emerald-300 border-emerald-500/25',
  },
  {
    id: 'endless',
    presetId: 'endless',
    gameMode: 'endless',
    nameZh: '无尽',
    nameEn: 'Endless',
    shortDescZh: '永不结束的饥荒沙盒模式。永远可以在绚丽之门复活。',
    shortDescEn: "A never-ending Don't Starve sandbox. Always revive at the Florid Postern.",
    detailDescZh:
      '永不结束的饥荒沙盒模式。世界不会因为所有冒险家死亡而重置，幽灵状态不消耗队友理智值，基础资源可持续再生，永远可以在绚丽之门复活。',
    detailDescEn:
      "A never-ending Don't Starve sandbox. The world never resets when all players die, ghosts do not drain sanity, basic resources regrow, and survivors can always revive at the Florid Postern.",
    tagsZh: ['世界永不重置', '无鬼魂掉理智', '绚丽之门复活'],
    tagsEn: ['No world reset', 'No ghost drain', 'Portal revive'],
    accentClass: 'bg-sky-500/14 border-sky-500/40',
    badgeClass: 'bg-sky-500/12 text-sky-700 dark:text-sky-300 border-sky-500/25',
  },
  {
    id: 'survival',
    presetId: 'survival',
    gameMode: 'survival',
    nameZh: '生存',
    nameEn: 'Survival',
    shortDescZh: '标准的饥荒体验。团队协作在严酷的永恒领域活下去。',
    shortDescEn: "The standard Don't Starve experience. Cooperate to endure the Constant.",
    detailDescZh:
      '经典的饥荒联机版生存模式。所有玩家阵亡后将开启世界重置倒计时，幽灵会降低附近队友的理智值，需要借助救赎之心、试金石或肉块雕像复活。',
    detailDescEn:
      "The classic Don't Starve Together experience. A reset timer starts if everyone dies, ghosts drain nearby survivors' sanity, and revival requires Telltale Hearts, Touch Stones, or Meat Effigies.",
    tagsZh: ['全员阵亡重置', '经典硬核规则', '原味生存'],
    tagsEn: ['Reset on wipe', 'Classic rules', 'Standard threat'],
    accentClass: 'bg-amber-500/12 border-amber-500/40',
    badgeClass: 'bg-amber-500/12 text-amber-700 dark:text-amber-300 border-amber-500/25',
  },
  {
    id: 'wilderness',
    presetId: 'wilderness',
    gameMode: 'wilderness',
    nameZh: '荒野',
    nameEn: 'Wilderness',
    shortDescZh: '冒险家随机降生在世界各处，死亡后重新选择角色重生。',
    shortDescEn: 'Survivors spawn in random locations and pick a new character upon death.',
    detailDescZh:
      '危机四伏的荒野模式！没有固定绚丽之门出生点，每位冒险家随机降生在世界各处；死亡后不会变成鬼魂，而是直接重新选择角色再次降生，世界永不重置。',
    detailDescEn:
      "It's dangerous out there! Survivors spawn scattered across the map instead of at a shared portal. On death, you return to character selection rather than becoming a ghost, and the world never resets.",
    tagsZh: ['随机地点出生', '死亡重选角色', '世界永不重置'],
    tagsEn: ['Random spawns', 'Respawn as new char', 'No world reset'],
    accentClass: 'bg-red-500/12 border-red-500/40',
    badgeClass: 'bg-rose-500/12 text-rose-700 dark:text-rose-300 border-rose-500/25',
  },
  {
    id: 'lightsout',
    presetId: 'lightsout',
    gameMode: 'survival',
    nameZh: '暗无天日',
    nameEn: 'Lights Out',
    shortDescZh: '永恒黑夜的残酷挑战，开局附带微弱照明补给。',
    shortDescEn: 'A pitch-black survival twist with permanent night and starter light supplies.',
    detailDescZh:
      '标准生存模式的永夜变体。世界永远处于黑夜之中，理智与查理的威胁时刻相伴；出生点将提供基础火源与补给箱，适合寻求高难度挑战的冒险家。',
    detailDescEn:
      "A dark twist on standard survival. The world stays in perpetual night with constant threats from Charlie and sanity drain, beginning at a darkness start location with basic light supplies.",
    tagsZh: ['仅夜晚循环', '黑暗开局补给', '高难挑战'],
    tagsEn: ['Night only', 'Darkness start', 'High challenge'],
    accentClass: 'bg-[var(--phase-night)]/18 border-[var(--phase-night)]/50',
    badgeClass: 'bg-violet-500/12 text-violet-700 dark:text-violet-300 border-violet-500/25',
  },
]

export function getPlaystyleById(id: string): PlaystyleDefinition {
  return PLAYSTYLES.find((p) => p.id === id) ?? PLAYSTYLES[1]
}

/**
 * Resolves an option value against its choices.
 * Legacy configs may store "default" for DST enabled/disabled options whose real
 * choices are "none" / "always"; DST treats "default" as the option's default.
 */
export function resolveOptionValue(opt: WorldOptionField, raw: unknown): string {
  if (raw === undefined || raw === null) return opt.default
  const str = String(raw)
  if (str === 'default' && opt.choices.every((ch) => ch.value !== 'default')) {
    return opt.default
  }
  return str
}

/**
 * Mirrors DST scripts/map/customize.lua option sorting within a group:
 * items with an explicit client `order` come first in ascending order,
 * followed by unordered items sorted by the active language label.
 */
export function sortClientOptions(options: WorldOptionField[], isEn: boolean): WorldOptionField[] {
  return [...options].sort((a, b) => {
    const ao = a.order ?? 0
    const bo = b.order ?? 0
    if (ao !== bo) {
      if (ao === 0) return 1
      if (bo === 0) return -1
      return ao - bo
    }
    const la = isEn ? a.label_en : a.label_zh
    const lb = isEn ? b.label_en : b.label_zh
    return la < lb ? -1 : la > lb ? 1 : 0
  })
}

export function extractPresetOverrides(lua: string): Record<string, string> {
  const block = lua.match(/overrides\s*=\s*\{([\s\S]*?)\}/)?.[1]
  if (!block) return {}
  const out: Record<string, string> = {}
  for (const m of block.matchAll(/([A-Za-z0-9_]+)\s*=\s*(?:"([^"]*)"|'([^']*)'|(true|false|-?\d+(?:\.\d+)?))/g)) {
    out[m[1]] = m[2] ?? m[3] ?? m[4] ?? ''
  }
  return out
}

export function countModifiedAgainstBaseline(
  schema: WorldOptionGroup[],
  draft: Record<string, string>,
  baseline: Record<string, string>,
): number {
  let count = 0
  for (const g of schema) {
    for (const opt of g.options) {
      const cur = resolveOptionValue(opt, draft[opt.key])
      const base = resolveOptionValue(opt, baseline[opt.key])
      if (cur !== base) count++
    }
  }
  return count
}

/**
 * Builds a minimal override diff relative to preset baseline so only user-changed keys
 * are sent on cluster creation, preserving all mandatory location defaults in the preset.
 */
export function buildOverrideDiff(
  schema: WorldOptionGroup[],
  draft: Record<string, string>,
  baseline: Record<string, string>,
): Record<string, string> {
  const diff: Record<string, string> = {}
  for (const g of schema) {
    for (const opt of g.options) {
      const cur = resolveOptionValue(opt, draft[opt.key])
      const base = resolveOptionValue(opt, baseline[opt.key])
      if (cur !== base) {
        diff[opt.key] = cur
      }
    }
  }
  return diff
}

/**
 * Suggests a valid directory cluster_id from a human-friendly room name.
 */
export function suggestClusterId(name: string, fallbackSuffix: string): string {
  const ascii = name
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9_-]+/g, '_')
    .replace(/^[_-]+|[_-]+$/g, '')
    .slice(0, 36)
  if (ascii.length >= 2) return ascii
  return `world_${fallbackSuffix}`
}

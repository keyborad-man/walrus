import React, { useEffect, useRef } from 'react'

/** Stylized jagged Don't Starve evergreen pine SVG path */
function pinePath(cx: number, baseY: number, h: number, w: number, lean: number): string {
  const topX = cx + lean
  const topY = baseY - h
  const t1Y = baseY - h * 0.72
  const t2Y = baseY - h * 0.48
  const t3Y = baseY - h * 0.24
  const t4Y = baseY - h * 0.08
  return [
    `M ${topX} ${topY}`,
    `L ${cx + w * 0.26 + lean * 0.6} ${t1Y}`,
    `L ${cx + w * 0.12 + lean * 0.5} ${t1Y - h * 0.03}`,
    `L ${cx + w * 0.42 + lean * 0.35} ${t2Y}`,
    `L ${cx + w * 0.22 + lean * 0.25} ${t2Y - h * 0.035}`,
    `L ${cx + w * 0.54 + lean * 0.12} ${t3Y}`,
    `L ${cx + w * 0.3 + lean * 0.08} ${t3Y - h * 0.03}`,
    `L ${cx + w * 0.58} ${t4Y}`,
    `L ${cx + w * 0.14} ${t4Y - h * 0.015}`,
    `L ${cx + w * 0.11} ${baseY}`,
    `L ${cx - w * 0.11} ${baseY}`,
    `L ${cx - w * 0.14} ${t4Y - h * 0.015}`,
    `L ${cx - w * 0.58} ${t4Y}`,
    `L ${cx - w * 0.3 + lean * 0.08} ${t3Y - h * 0.03}`,
    `L ${cx - w * 0.54 + lean * 0.12} ${t3Y}`,
    `L ${cx - w * 0.22 + lean * 0.25} ${t2Y - h * 0.035}`,
    `L ${cx - w * 0.42 + lean * 0.35} ${t2Y}`,
    `L ${cx - w * 0.12 + lean * 0.5} ${t1Y - h * 0.03}`,
    `L ${cx - w * 0.26 + lean * 0.6} ${t1Y}`,
    'Z',
  ].join(' ')
}

const FAR_PINES = [
  pinePath(70, 320, 175, 76, -8),
  pinePath(165, 320, 215, 88, 6),
  pinePath(275, 320, 150, 68, -5),
  pinePath(370, 320, 118, 56, 4),
  pinePath(1075, 320, 122, 58, -5),
  pinePath(1170, 320, 165, 74, 7),
  pinePath(1275, 320, 220, 92, -9),
  pinePath(1375, 320, 185, 80, 5),
].join(' ')

const MID_PINES = [
  pinePath(30, 320, 240, 108, 12),
  pinePath(130, 320, 195, 92, -10),
  pinePath(235, 320, 155, 78, 8),
  pinePath(335, 320, 98, 52, -4),
  pinePath(1110, 320, 104, 54, 5),
  pinePath(1210, 320, 168, 82, -11),
  pinePath(1315, 320, 205, 96, 9),
  pinePath(1415, 320, 245, 110, -12),
].join(' ')

const NEAR_PINES = [
  pinePath(-15, 320, 290, 130, 14),
  pinePath(95, 320, 225, 104, -12),
  pinePath(195, 320, 145, 72, 9),
  pinePath(1245, 320, 150, 74, -8),
  pinePath(1345, 320, 232, 108, 11),
  pinePath(1450, 320, 295, 132, -15),
].join(' ')

const STARS = [
  { cx: 140, cy: 62, r: 1.1, dur: '4.5s', delay: '0s' },
  { cx: 290, cy: 112, r: 0.9, dur: '5.8s', delay: '-1.8s' },
  { cx: 430, cy: 48, r: 1.2, dur: '4.2s', delay: '-3.1s' },
  { cx: 990, cy: 56, r: 1.1, dur: '5.2s', delay: '-0.9s' },
  { cx: 1140, cy: 96, r: 1.3, dur: '4.7s', delay: '-2.6s' },
  { cx: 1290, cy: 64, r: 0.9, dur: '6.1s', delay: '-4.2s' },
  { cx: 215, cy: 165, r: 0.85, dur: '5.5s', delay: '-2.0s' },
  { cx: 1225, cy: 158, r: 0.9, dur: '5.0s', delay: '-1.3s' },
]

const EMBERS = [
  { left: '46%', bottom: '18px', size: 2.5, dur: '6.8s', delay: '0s', dx1: '12px', dx2: '-14px', op: 0.8, warm: true },
  { left: '51%', bottom: '12px', size: 3, dur: '7.6s', delay: '-2.1s', dx1: '-10px', dx2: '16px', op: 0.75, warm: false },
  { left: '54%', bottom: '20px', size: 2, dur: '6.2s', delay: '-4.2s', dx1: '8px', dx2: '-18px', op: 0.7, warm: true },
  { left: '42%', bottom: '14px', size: 2, dur: '8.4s', delay: '-1.5s', dx1: '-14px', dx2: '10px', op: 0.55, warm: false },
  { left: '58%', bottom: '16px', size: 2.5, dur: '7.9s', delay: '-5.4s', dx1: '14px', dx2: '-8px', op: 0.6, warm: true },
  { left: '49%', bottom: '8px', size: 2, dur: '5.8s', delay: '-3.1s', dx1: '6px', dx2: '18px', op: 0.72, warm: false },
  { left: '36%', bottom: '10px', size: 2, dur: '9.2s', delay: '-6.0s', dx1: '10px', dx2: '-12px', op: 0.45, warm: true },
  { left: '64%', bottom: '12px', size: 2, dur: '8.8s', delay: '-0.8s', dx1: '-12px', dx2: '14px', op: 0.45, warm: false },
]

const FIREFLIES = [
  { left: '14%', bottom: '22%', size: 3.5, dur: '5.4s', delay: '0s', fx: '10px', fy: '-12px' },
  { left: '21%', bottom: '15%', size: 2.5, dur: '6.6s', delay: '-2.3s', fx: '-8px', fy: '-10px' },
  { left: '8%', bottom: '30%', size: 2.5, dur: '7.1s', delay: '-4.0s', fx: '12px', fy: '8px' },
  { left: '84%', bottom: '24%', size: 3.5, dur: '5.9s', delay: '-1.2s', fx: '-12px', fy: '-10px' },
  { left: '78%', bottom: '14%', size: 2.5, dur: '6.8s', delay: '-3.6s', fx: '9px', fy: '-14px' },
  { left: '91%', bottom: '32%', size: 2.5, dur: '7.4s', delay: '-5.1s', fx: '-10px', fy: '10px' },
]

interface AuthCampfireBackdropProps {
  /** Container ref used for subtle pointer-fine parallax */
  containerRef: React.RefObject<HTMLElement | null>
}

/**
 * Pure offline SVG/CSS Don't Starve Together atmosphere:
 * layered Constant evergreens, stone firepit glow, rising embers, forest fireflies,
 * and crescent moon starlight. Adapts cleanly to dark obsidian and light ink themes.
 */
export function AuthCampfireBackdrop({ containerRef }: AuthCampfireBackdropProps) {
  const treesRef = useRef<SVGSVGElement | null>(null)
  const glowRef = useRef<HTMLDivElement | null>(null)

  useEffect(() => {
    const container = containerRef.current
    if (!container) return
    const prefersReduced = window.matchMedia('(prefers-reduced-motion: reduce)').matches
    const hasFinePointer = window.matchMedia('(hover: hover) and (pointer: fine)').matches
    if (prefersReduced || !hasFinePointer) return

    const onMove = (e: PointerEvent) => {
      const rect = container.getBoundingClientRect()
      if (!rect.width || !rect.height) return
      const nx = ((e.clientX - rect.left) / rect.width - 0.5) * 2
      const ny = ((e.clientY - rect.top) / rect.height - 0.5) * 2
      if (treesRef.current) {
        treesRef.current.style.transform = `translate3d(${(nx * -8).toFixed(1)}px, ${(ny * -3).toFixed(1)}px, 0)`
      }
      if (glowRef.current) {
        glowRef.current.style.transform = `translate3d(${(nx * 14).toFixed(1)}px, 0, 0)`
      }
    }

    const onLeave = () => {
      if (treesRef.current) treesRef.current.style.transform = 'translate3d(0, 0, 0)'
      if (glowRef.current) glowRef.current.style.transform = 'translate3d(0, 0, 0)'
    }

    container.addEventListener('pointermove', onMove)
    container.addEventListener('pointerleave', onLeave)
    return () => {
      container.removeEventListener('pointermove', onMove)
      container.removeEventListener('pointerleave', onLeave)
    }
  }, [containerRef])

  return (
    <div aria-hidden="true" className="pointer-events-none absolute inset-0 overflow-hidden auth-enter-scene">
      {/* 1px top horizon ember sweep on mount */}
      <div className="absolute inset-x-0 top-0 h-px overflow-hidden">
        <div
          className="auth-horizon-beam h-full w-1/2 mx-auto"
          style={{ background: 'linear-gradient(90deg, transparent, var(--dst-beam), transparent)' }}
        />
      </div>

      {/* Upper sky stars & crescent moon */}
      <svg
        viewBox="0 0 1440 260"
        preserveAspectRatio="xMidYMin slice"
        className="absolute inset-x-0 top-0 w-full h-[260px]"
        focusable="false"
      >
        {STARS.map((s, i) => (
          <circle
            key={i}
            cx={s.cx}
            cy={s.cy}
            r={s.r}
            fill="var(--dst-star)"
            className="auth-star-twinkle"
            style={{ '--dur': s.dur, '--delay': s.delay } as React.CSSProperties}
          />
        ))}
        <g transform="translate(230, 74)" opacity="0.9">
          <circle
            cx="0"
            cy="0"
            r="28"
            fill="none"
            stroke="var(--dst-moon)"
            strokeWidth="0.7"
            strokeDasharray="2 5"
            opacity="0.45"
          />
          <path d="M 4 -15 A 15 15 0 1 0 15 8 A 12 12 0 1 1 4 -15 Z" fill="var(--dst-moon)" />
        </g>
      </svg>

      {/* Campfire warm breathing glow at bottom-center */}
      <div ref={glowRef} className="auth-parallax absolute inset-x-0 bottom-[-90px] mx-auto w-[780px] max-w-[140vw] h-[360px]">
        <div
          className="h-full w-full auth-campfire-breathe"
          style={{
            background:
              'radial-gradient(ellipse 52% 62% at 50% 82%, var(--dst-ember-core) 0%, var(--dst-ember-outer) 45%, transparent 74%)',
          }}
        />
      </div>

      {/* Layered Constant evergreen forest silhouettes & stone firepit */}
      <svg
        ref={treesRef}
        viewBox="0 0 1440 320"
        preserveAspectRatio="xMidYMax slice"
        className="auth-parallax absolute -inset-x-4 -bottom-1.5 w-[calc(100%+2rem)] h-[266px] sm:h-[326px]"
        focusable="false"
      >
        <path
          d="M 0 320 L 0 268 Q 220 248 450 278 Q 720 302 990 278 Q 1220 250 1440 266 L 1440 320 Z"
          fill="var(--dst-tree-far)"
        />
        <path d={FAR_PINES} fill="var(--dst-tree-far)" />

        <path
          d="M 0 320 L 0 282 Q 260 272 520 296 Q 720 308 920 296 Q 1180 270 1440 284 L 1440 320 Z"
          fill="var(--dst-tree-mid)"
          stroke="var(--dst-tree-stroke)"
          strokeWidth="1"
        />
        <path
          d={MID_PINES}
          fill="var(--dst-tree-mid)"
          stroke="var(--dst-tree-stroke)"
          strokeWidth="0.9"
          strokeLinejoin="round"
        />

        <path
          d={NEAR_PINES}
          fill="var(--dst-tree-near)"
          stroke="var(--dst-tree-stroke)"
          strokeWidth="1"
          strokeLinejoin="round"
        />
        <path
          d="M 0 320 L 0 298 Q 300 292 640 310 L 688 307 L 702 299 L 720 303 L 738 299 L 752 307 L 800 310 Q 1140 292 1440 298 L 1440 320 Z"
          fill="var(--dst-tree-near)"
        />

        {/* Stone firepit & subtle breathing flame silhouette */}
        <g transform="translate(720, 304)">
          <ellipse cx="0" cy="4" rx="22" ry="4.5" fill="var(--dst-ember-core)" opacity="0.9" />
          <path
            d="M -20 4 L -13 -1 L -5 2 L 4 -2 L 14 0 L 20 4 Z"
            fill="var(--dst-tree-near)"
            stroke="var(--dst-tree-stroke)"
            strokeWidth="0.9"
          />
          <path
            d="M -5 0 C -6 -6 -1 -9 0 -13 C 2 -9 6 -5 5 0 C 2 -1 -2 -1 -5 0 Z"
            fill="var(--dst-spark)"
            opacity="0.45"
            className="auth-campfire-breathe"
          />
        </g>
      </svg>

      {/* Gently rising campfire sparks */}
      {EMBERS.map((e, i) => (
        <span
          key={i}
          className="auth-ember-particle"
          style={
            {
              left: e.left,
              bottom: e.bottom,
              width: `${e.size}px`,
              height: `${e.size}px`,
              background: e.warm ? 'var(--dst-spark-warm)' : 'var(--dst-spark)',
              boxShadow: `0 0 ${e.size * 3}px ${e.warm ? 'var(--dst-spark-warm)' : 'var(--dst-spark)'}`,
              '--dur': e.dur,
              '--delay': e.delay,
              '--drift-x1': e.dx1,
              '--drift-x2': e.dx2,
              '--ember-op': e.op,
            } as React.CSSProperties
          }
        />
      ))}

      {/* Quiet forest fireflies on outer flanks (away from center form) */}
      {FIREFLIES.map((f, i) => (
        <span
          key={i}
          className="auth-firefly hidden sm:block"
          style={
            {
              left: f.left,
              bottom: f.bottom,
              width: `${f.size}px`,
              height: `${f.size}px`,
              background: 'var(--dst-spark)',
              boxShadow: '0 0 10px var(--dst-spark)',
              '--dur': f.dur,
              '--delay': f.delay,
              '--fx': f.fx,
              '--fy': f.fy,
            } as React.CSSProperties
          }
        />
      ))}
    </div>
  )
}

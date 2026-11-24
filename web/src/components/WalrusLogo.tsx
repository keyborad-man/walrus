import React from 'react'

interface WalrusLogoProps extends React.SVGProps<SVGSVGElement> {
  className?: string
}

/**
 * Minimalist geometric Walrus wearing a Tam o' Shanter beret.
 * Uses currentColor + evenodd cutouts so it adapts cleanly to dark/light badges.
 */
export function WalrusLogo({ className, ...props }: WalrusLogoProps) {
  return (
    <svg
      xmlns="http://www.w3.org/2000/svg"
      viewBox="0 0 24 24"
      fill="none"
      aria-hidden="true"
      className={className}
      {...props}
    >
      {/* Beret pom-pom (toorie) */}
      <circle cx="12.5" cy="2.75" r="1.2" fill="currentColor" />
      {/* Jaunty Scottish beret */}
      <path
        d="M5.2 8.2C4.0 6.8 4.7 4.4 9.0 3.6C13.6 2.8 18.8 4.0 19.9 6.0C20.5 7.1 19.6 8.1 18.6 8.2Q12 7.1 5.2 8.2Z"
        fill="currentColor"
      />
      {/* Walrus head & plump whisker pads with evenodd eye/nose cutouts */}
      <path
        fillRule="evenodd"
        clipRule="evenodd"
        d="M6.3 9.3Q12 8.3 17.7 9.3L18.4 12.2C19.8 13.0 20.1 15.6 18.2 16.9C16.6 17.9 13.8 17.5 12.0 16.0C10.2 17.5 7.4 17.9 5.8 16.9C3.9 15.6 4.2 13.0 5.6 12.2L6.3 9.3ZM10.15 11.7a1.05 1.05 0 1 0 -2.1 0a1.05 1.05 0 1 0 2.1 0ZM15.95 11.7a1.05 1.05 0 1 0 -2.1 0a1.05 1.05 0 1 0 2.1 0ZM13.25 13.55a1.25 0.8 0 1 0 -2.5 0a1.25 0.8 0 1 0 2.5 0Z"
        fill="currentColor"
      />
      {/* Twin tusks */}
      <path
        d="M7.2 17.9C7.1 19.9 7.6 21.6 8.3 22.4C9.1 21.5 9.6 19.8 9.6 17.8C8.8 18.1 8.0 18.1 7.2 17.9Z"
        fill="currentColor"
      />
      <path
        d="M16.8 17.9C16.9 19.9 16.4 21.6 15.7 22.4C14.9 21.5 14.4 19.8 14.4 17.8C15.2 18.1 16.0 18.1 16.8 17.9Z"
        fill="currentColor"
      />
    </svg>
  )
}

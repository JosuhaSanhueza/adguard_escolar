import React, { memo } from 'react';

type Props = {
    className?: string;
};

/**
 * EscolarApp Shield logo.  "Escolar" uses currentColor so the logo follows the
 * light and dark themes without inverting the brand colors.
 */
export const Logo = memo(({ className }: Props) => {
    return (
        <svg
            xmlns="http://www.w3.org/2000/svg"
            width="290"
            height="64"
            viewBox="0 0 290 64"
            className={className}
            role="img"
            aria-label="EscolarApp Shield">
            <defs>
                <linearGradient id="escolarapp-shield-gradient" x1="0" y1="0" x2="1" y2="1">
                    <stop offset="0" stopColor="#4F46E5" />
                    <stop offset="1" stopColor="#0EA5E9" />
                </linearGradient>
            </defs>
            <path
                d="M32 3 L56 11.5 V29 C56 44.5 46.2 55.4 32 61 C17.8 55.4 8 44.5 8 29 V11.5 Z"
                fill="url(#escolarapp-shield-gradient)"
            />
            <path d="M32 3 L56 11.5 V29 C56 44.5 46.2 55.4 32 61 Z" fill="#fff" opacity=".08" />
            <path
                transform="translate(17.6 15.4) scale(1.2)"
                fill="#fff"
                d="M12 3L1 9l11 6 9-4.91V17h2V9L12 3zM5 13.18v4L12 21l7-3.82v-4L12 17l-7-3.82z"
            />
            <text
                x="70"
                y="41"
                fontFamily="'Plus Jakarta Sans', Outfit, 'Segoe UI', Roboto, Arial, sans-serif"
                fontSize="26"
                fontWeight="800"
                letterSpacing="-0.5">
                <tspan fill="currentColor">Escolar</tspan>
                <tspan className="logo__app" fill="#4F46E5">
                    App
                </tspan>
                <tspan className="logo__product" fill="#64748B" fontWeight="600" dx="7">
                    Shield
                </tspan>
            </text>
        </svg>
    );
});

Logo.displayName = 'Logo';

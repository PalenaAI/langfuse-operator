import { defineConfig } from 'vitepress'

export default defineConfig({
  title: 'Langfuse Operator',
  description: 'Kubernetes operator for deploying and managing production-ready Langfuse instances',

  head: [
    ['link', { rel: 'icon', type: 'image/svg+xml', href: '/logo.svg' }],
  ],

  themeConfig: {
    logo: '/logo.svg',

    nav: [
      { text: 'Guide', link: '/guide/what-is-langfuse-operator' },
      { text: 'Reference', link: '/reference/langfuseinstance' },
      {
        text: 'Links',
        items: [
          { text: 'GitHub', link: 'https://github.com/PalenaAI/langfuse-operator' },
          { text: 'Langfuse', link: 'https://langfuse.com' },
        ],
      },
    ],

    sidebar: {
      '/guide/': [
        {
          text: 'Introduction',
          items: [
            { text: 'What is Langfuse Operator?', link: '/guide/what-is-langfuse-operator' },
            { text: 'Architecture', link: '/guide/architecture' },
          ],
        },
        {
          text: 'Getting Started',
          items: [
            { text: 'Installation', link: '/guide/installation' },
            { text: 'Quick Start', link: '/guide/quickstart' },
          ],
        },
        {
          text: 'Configuration',
          items: [
            { text: 'Database', link: '/guide/database' },
            { text: 'ClickHouse', link: '/guide/clickhouse' },
            { text: 'Redis', link: '/guide/redis' },
            { text: 'Blob Storage', link: '/guide/blob-storage' },
            { text: 'Authentication', link: '/guide/authentication' },
            { text: 'Networking', link: '/guide/networking' },
            { text: 'Observability', link: '/guide/observability' },
          ],
        },
        {
          text: 'Operations',
          items: [
            { text: 'Upgrades', link: '/guide/upgrades' },
            { text: 'Secret Management', link: '/guide/secrets' },
            { text: 'Multi-Tenancy', link: '/guide/multi-tenancy' },
          ],
        },
      ],
      '/reference/': [
        {
          text: 'Custom Resources',
          items: [
            { text: 'LangfuseInstance', link: '/reference/langfuseinstance' },
            { text: 'LangfuseOrganization', link: '/reference/langfuseorganization' },
            { text: 'LangfuseProject', link: '/reference/langfuseproject' },
          ],
        },
      ],
    },

    socialLinks: [
      { icon: 'github', link: 'https://github.com/PalenaAI/langfuse-operator' },
    ],

    editLink: {
      pattern: 'https://github.com/PalenaAI/langfuse-operator/edit/main/docs/:path',
      text: 'Edit this page on GitHub',
    },

    footer: {
      message: 'Released under the Apache 2.0 License.',
      copyright: 'Copyright 2026 bitkaio LLC',
    },

    search: {
      provider: 'local',
    },
  },
})

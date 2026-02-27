// @ts-check
import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';

export default defineConfig({
	integrations: [
		starlight({
			title: 'society',
			description: 'Agent-to-Agent orchestration over JSON-RPC 2.0',
			social: [{ icon: 'github', label: 'GitHub', href: 'https://github.com/lucharo/society' }],
			customCss: ['./src/styles/custom.css'],
			sidebar: [
				{
					label: 'Getting Started',
					items: [
						{ label: 'Introduction', slug: 'getting-started/introduction' },
						{ label: 'Installation', slug: 'getting-started/installation' },
						{ label: 'Quickstart', slug: 'getting-started/quickstart' },
					],
				},
				{
					label: 'Guides',
					items: [
						{ label: 'Creating Agents', slug: 'guides/creating-agents' },
						{ label: 'Connecting Machines', slug: 'guides/connecting-machines' },
						{ label: 'Daemon Mode', slug: 'guides/daemon' },
						{ label: 'MCP Integration', slug: 'guides/mcp' },
					],
				},
				{
					label: 'Transports',
					items: [
						{ label: 'HTTP (Local)', slug: 'transports/http' },
						{ label: 'SSH', slug: 'transports/ssh' },
						{ label: 'Docker', slug: 'transports/docker' },
						{ label: 'STDIO', slug: 'transports/stdio' },
					],
				},
				{
					label: 'Reference',
					items: [
						{ label: 'CLI Commands', slug: 'reference/cli' },
						{ label: 'Agent Config', slug: 'reference/agent-config' },
						{ label: 'Registry', slug: 'reference/registry' },
						{ label: 'A2A Protocol', slug: 'reference/protocol' },
					],
				},
			],
		}),
	],
});

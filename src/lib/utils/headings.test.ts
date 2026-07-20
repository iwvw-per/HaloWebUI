import { Marked } from 'marked';
import { describe, expect, it } from 'vitest';

import markedExtension from '$lib/utils/marked/extension';
import { extractHeadings } from './headings';

function createParser() {
	const parser = new Marked();
	parser.use(markedExtension());
	return parser;
}

describe('extractHeadings', () => {
	it('returns an empty list when markdown has no headings or code blocks', () => {
		expect(extractHeadings(createParser().lexer('plain text only'), 'message-1')).toEqual([]);
	});

	it('extracts top-level headings with stable ids', () => {
		expect(extractHeadings(createParser().lexer('# Intro\n\n## Details'), 'message-1')).toEqual([
			{ depth: 1, text: 'Intro', id: 'heading-message-1-0' },
			{ depth: 2, text: 'Details', id: 'heading-message-1-1' }
		]);
	});

	it('extracts nested headings inside blockquotes and lists', () => {
		const markdown = `
> ## Quote Heading
>
> Content

- item
  ### Nested Heading
`;

		expect(extractHeadings(createParser().lexer(markdown), 'message-2')).toEqual([
			{ depth: 2, text: 'Quote Heading', id: 'heading-message-2-1-0' },
			{ depth: 3, text: 'Nested Heading', id: 'heading-message-2-2-0-1' }
		]);
	});

	it('keeps duplicate heading text unique by token path', () => {
		expect(extractHeadings(createParser().lexer('## Repeat\n\n## Repeat'), 'message-3')).toEqual([
			{ depth: 2, text: 'Repeat', id: 'heading-message-3-0' },
			{ depth: 2, text: 'Repeat', id: 'heading-message-3-1' }
		]);
	});

	it('extracts code blocks as outline items', () => {
		const markdown = '```sql\nselect 1;\n```\n\n```dockerfile\nFROM alpine:3.18\n```';

		expect(extractHeadings(createParser().lexer(markdown), 'message-4')).toEqual([
			{ depth: 2, text: 'SQL', id: 'heading-message-4-0' },
			{ depth: 2, text: 'Dockerfile', id: 'heading-message-4-2' }
		]);
	});

	it('extracts headings and code blocks from details content', () => {
		const markdown = [
			'<details>',
			'<summary>More</summary>',
			'',
			'## Deep Section',
			'',
			'```yaml',
			'app:',
			'  name: demo',
			'```',
			'</details>'
		].join('\n');

		expect(extractHeadings(createParser().lexer(markdown), 'message-5')).toEqual([
			{ depth: 2, text: 'Deep Section', id: 'heading-message-5-0-0' },
			{ depth: 2, text: 'YAML', id: 'heading-message-5-0-1' }
		]);
	});

	it('falls back to a generic label for unlabeled code blocks', () => {
		expect(extractHeadings(createParser().lexer('```\nplain text\n```'), 'message-6')).toEqual([
			{ depth: 2, text: 'Code Block', id: 'heading-message-6-0' }
		]);
	});
});

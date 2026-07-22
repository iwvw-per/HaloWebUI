import { spawn } from 'node:child_process';
import { once } from 'node:events';
import { resolve } from 'node:path';

const root = resolve(import.meta.dirname, '..');
const isWindows = process.platform === 'win32';
const spawnCommand = (name, args, options) =>
	spawn(isWindows ? process.env.ComSpec ?? 'cmd.exe' : name, isWindows ? ['/d', '/s', '/c', name, ...args] : args, options);

const children = [
	spawnCommand('npm', ['run', 'sync:static'], { cwd: root, stdio: 'inherit' }),
];

const stop = () => {
	for (const child of children) {
		if (!child.killed) child.kill('SIGTERM');
	}
};

const fail = (child, label) => {
	child.once('exit', (code) => {
		if (code && !process.exitCode) {
			console.error(`${label} exited with code ${code}`);
			process.exitCode = code;
			stop();
		}
	});
};

fail(children[0], 'static sync');
await once(children[0], 'exit');

const backend = spawnCommand('go', ['run', './cmd/halowebui'], {
	cwd: resolve(root, 'backend'),
	stdio: 'inherit',
	env: {
		...process.env,
		PORT: process.env.PORT ?? '8080',
		FRONTEND_DIR: process.env.FRONTEND_DIR ?? resolve(root, 'build')
	}
});
children.push(backend);
fail(backend, 'backend');

const frontend = spawnCommand('npm', ['exec', '--', 'vite', 'dev', '--host', '--port', '5180'], {
	cwd: root,
	stdio: 'inherit'
});
children.push(frontend);
fail(frontend, 'frontend');

process.once('SIGINT', () => {
	stop();
	process.exitCode = 0;
});
process.once('SIGTERM', () => {
	stop();
	process.exitCode = 0;
});

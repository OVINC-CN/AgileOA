import { createHash } from 'node:crypto';
import { existsSync, mkdtempSync, rmSync } from 'node:fs';
import {
  access,
  copyFile,
  mkdir,
  readdir,
  readFile,
  rename,
  rm,
  writeFile,
} from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { dirname, join, relative, resolve, sep } from 'node:path';
import { fileURLToPath, URL } from 'node:url';

import { fileViewerRenderers } from '@file-viewer/vite-plugin';
import tailwindcss from '@tailwindcss/vite';
import react from '@vitejs/plugin-react';
import type { Plugin, UserConfig } from 'vite';
import { defineConfig } from 'vitest/config';

import {
  deploymentFields,
  extraAssetsDirectory,
  fileViewerAssetsDirectory,
} from './src/lib/deployment-config.ts';

const productionPasswordLoginEnabled = false;
const fileViewerStagingDirectory = `${extraAssetsDirectory}/.file-viewer`;

async function hashAssetDirectory(directory: string): Promise<string> {
  // 按相对路径排序，哈希只取决于路径和内容。
  const entries = await readdir(directory, { recursive: true, withFileTypes: true });
  const paths = entries
    .filter((entry) => entry.isFile())
    .map((entry) => relative(directory, join(entry.parentPath, entry.name)).split(sep).join('/'))
    .sort();
  const hash = createHash('sha256');
  for (const path of paths) {
    const contentHash = createHash('sha256').update(await readFile(join(directory, path)));
    hash.update(JSON.stringify([path, contentHash.digest('hex')]));
  }
  return hash.digest('hex');
}

async function publishFileViewerAssets(outputDirectory: string): Promise<string> {
  const stagingDirectory = resolve(outputDirectory, fileViewerStagingDirectory);
  await access(join(stagingDirectory, 'flyfish-viewer-assets.json'));

  // 补齐随预览资源分发的许可文件，再对整组资源计算哈希。
  const notices = [
    ['core', 'LICENSE'],
    ['renderer-image', 'THIRD_PARTY_NOTICES.md'],
    ['renderer-pdf', 'THIRD_PARTY_NOTICES.md'],
    ['renderer-spreadsheet', 'THIRD_PARTY_NOTICES.md'],
  ] as const;
  for (const [name, notice] of notices) {
    const entry = import.meta.resolve(`@file-viewer/${name}`);
    const target = join(stagingDirectory, 'licenses', name, notice);
    await mkdir(dirname(target), { recursive: true });
    await copyFile(new URL(`../${notice}`, entry), target);
  }

  const hash = await hashAssetDirectory(stagingDirectory);
  const assetPath = `${fileViewerAssetsDirectory}/${hash.slice(0, 12)}`;
  const target = resolve(outputDirectory, assetPath);
  if (existsSync(target)) {
    if ((await hashAssetDirectory(target)) !== hash) {
      throw new Error('同名预览资源目录的内容不一致');
    }
    await rm(stagingDirectory, { recursive: true });
  } else {
    await mkdir(dirname(target), { recursive: true });
    await rename(stagingDirectory, target);
  }
  return assetPath;
}

function buildRuntimeConfig(): Plugin {
  let outputDirectory: string | undefined;
  return {
    name: 'build-runtime-config',
    apply: 'build',
    async writeBundle(outputOptions) {
      if (!outputOptions.dir) {
        this.error('无法确定构建产物目录');
      }
      // 清理中断构建留下的暂存资源，Flyfish 随后在 closeBundle 中复制新资源。
      await rm(resolve(outputOptions.dir, fileViewerStagingDirectory), {
        recursive: true,
        force: true,
      });
      outputDirectory = outputOptions.dir;
    },
    closeBundle: {
      order: 'post',
      async handler() {
        if (!outputDirectory) return;

        // 等 Flyfish 完成复制，将本次资源路径写入与页面对应的配置。
        const fileViewerAssetsPath = await publishFileViewerAssets(outputDirectory);
        const configPath = resolve(outputDirectory, 'config.js');
        const source = await readFile(configPath, 'utf8');
        const passwordLoginEnabledPattern =
          /^([ \t]*passwordLoginEnabled:[ \t]*)(?:true|false)(?=,)/gm;
        if ([...source.matchAll(passwordLoginEnabledPattern)].length !== 1) {
          this.error('构建配置必须包含唯一的 passwordLoginEnabled 布尔字段');
        }

        // 写入生产配置值并校验字段，避免替换失败后继续发布。
        let productionConfig = source.replace(
          passwordLoginEnabledPattern,
          (_match, prefix: string) => `${prefix}${String(productionPasswordLoginEnabled)}`,
        );
        const overrides = {
          apiBaseURL: process.env.API_BASE_URL?.trim() ?? '',
          contactEmail: process.env.CONTACT_EMAIL?.trim(),
          fileViewerAssetsPath,
        };
        for (const field of Object.keys(overrides) as (keyof typeof overrides)[]) {
          const pattern = new RegExp(
            String.raw`^([ \t]*${field}:[ \t]*)(?:'(?:\\.|[^'\\\r\n])*'|"(?:\\.|[^"\\\r\n])*")(?=,)`,
            'gm',
          );
          if ([...productionConfig.matchAll(pattern)].length !== 1) {
            this.error(`构建配置必须包含唯一的 ${field} 字符串字段`);
          }
          const value = overrides[field];
          if (value === undefined) {
            continue;
          }
          if (!deploymentFields[field].safeParse(value).success) {
            this.error(`构建配置中的 ${field} 格式无效`);
          }
          productionConfig = productionConfig.replace(
            pattern,
            (_match, prefix: string) => `${prefix}${JSON.stringify(value)}`,
          );
        }

        // 按最终配置内容生成文件名，并同步更新 HTML 中的独立脚本引用。
        const configHash = createHash('sha256').update(productionConfig).digest('hex').slice(0, 12);
        const configFilename = `config.${configHash}.js`;
        const htmlPath = resolve(outputDirectory, 'index.html');
        const html = await readFile(htmlPath, 'utf8');
        const configReference = 'src="/config.js"';
        if (html.split(configReference).length !== 2) {
          this.error('构建页面必须包含唯一的 config.js 脚本引用');
        }

        // 先写入配置再更新页面，移除不再使用的无哈希产物。
        await writeFile(resolve(outputDirectory, configFilename), productionConfig);
        await writeFile(htmlPath, html.replace(configReference, `src="/${configFilename}"`));
        await rm(configPath);
      },
    },
  };
}

const config = defineConfig({
  plugins: [react(), tailwindcss()],
  resolve: {
    alias: [
      { find: '@', replacement: fileURLToPath(new URL('./src', import.meta.url)) },
      {
        // 使用 COS SDK 的源码入口，使浏览器构建使用锁定的 XML 解析器版本。
        find: /^cos-js-sdk-v5$/u,
        replacement: fileURLToPath(import.meta.resolve('cos-js-sdk-v5/index.js')),
      },
    ],
  },
  server: { port: 5173, strictPort: true },
  build: { sourcemap: false },
  test: {
    pool: 'vmThreads',
    maxWorkers: '100%',
    environment: 'jsdom',
    setupFiles: ['./src/test/setup.ts'],
    clearMocks: true,
    restoreMocks: true,
    unstubGlobals: true,
    css: false,
  },
});

export default defineConfig(({ command, mode }): UserConfig => {
  if (mode === 'test') return config;

  // 开发服务独立生成资源，避免并行启动或升级依赖时复用旧文件。
  const previewAssets =
    command === 'serve' ? mkdtempSync(join(tmpdir(), 'agileoa-file-viewer-')) : undefined;
  return {
    ...config,
    plugins: [
      ...(config.plugins ?? []),
      fileViewerRenderers({
        renderers: ['word', 'spreadsheet', 'presentation', 'pdf', 'image'],
        inject: false,
        chunkStrategy: 'none',
        copyAssets: {
          baseDir: previewAssets ? fileViewerAssetsDirectory : fileViewerStagingDirectory,
          ...(previewAssets ? { publicDir: previewAssets } : {}),
        },
      }),
      buildRuntimeConfig(),
      previewAssets
        ? {
            name: 'clean-preview-assets',
            configureServer(server) {
              server.httpServer?.once('close', () => {
                rmSync(previewAssets, { recursive: true, force: true });
              });
            },
          }
        : undefined,
    ],
  };
});

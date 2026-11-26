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
import { minifySync, type Plugin, type UserConfig } from 'vite';
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

        // 校验生产构建覆盖值，并在原配置求值后合并。
        const overrides = {
          apiBaseURL: process.env.API_BASE_URL?.trim() ?? '',
          contactEmail: process.env.CONTACT_EMAIL?.trim(),
          fileViewerAssetsPath,
        };
        for (const field of Object.keys(overrides) as (keyof typeof overrides)[]) {
          const value = overrides[field];
          if (value !== undefined && !deploymentFields[field].safeParse(value).success) {
            this.error(`构建配置中的 ${field} 格式无效`);
          }
        }

        // 保留原监控对象的属性，应用 ID 由显式设置的环境变量覆盖。
        const aegisId = process.env.AEGIS_ID?.trim();
        const aegisOverride =
          aegisId === undefined
            ? ''
            : `aegis: { ...globalThis.AGILE_WORKSPACE_CONFIG.aegis, id: ${JSON.stringify(aegisId)} },`;
        const fields = JSON.stringify({
          ...overrides,
          passwordLoginEnabled: productionPasswordLoginEnabled,
        });
        const productionConfig = `${source}\n;globalThis.AGILE_WORKSPACE_CONFIG = {
  ...globalThis.AGILE_WORKSPACE_CONFIG,
  ...${fields},
  ${aegisOverride}
};\n`;

        // 去除发布配置中的注释，保留字段名和可读格式。
        const { code, errors } = minifySync(configPath, productionConfig, {
          compress: false,
          mangle: false,
          codegen: { removeWhitespace: false, legalComments: 'none' },
        });
        if (errors.length > 0) {
          this.error('生产配置脚本语法无效');
        }

        // 按发布配置的实际内容生成文件名，并更新 HTML 中的独立脚本引用。
        const configHash = createHash('sha256').update(code).digest('hex').slice(0, 12);
        const configFilename = `config.${configHash}.js`;
        const htmlPath = resolve(outputDirectory, 'index.html');
        const html = await readFile(htmlPath, 'utf8');
        const configReference = 'src="/config.js"';
        if (html.split(configReference).length !== 2) {
          this.error('构建页面必须包含唯一的 config.js 脚本引用');
        }

        // 先写入配置再更新页面，移除不再使用的无哈希产物。
        await writeFile(resolve(outputDirectory, configFilename), code);
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

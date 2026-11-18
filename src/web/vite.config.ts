import { createHash } from 'node:crypto';
import { readFile, rm, writeFile } from 'node:fs/promises';
import { resolve } from 'node:path';
import { fileURLToPath, URL } from 'node:url';

import tailwindcss from '@tailwindcss/vite';
import react from '@vitejs/plugin-react';
import { defineConfig } from 'vitest/config';

import { deploymentFields } from './src/lib/deployment-config.ts';

const productionPasswordLoginEnabled = false;

export default defineConfig({
  plugins: [
    react(),
    tailwindcss(),
    {
      name: 'build-runtime-config',
      apply: 'build',
      async writeBundle(outputOptions) {
        // 校验产物目录及配置字段，避免替换失败后继续发布。
        if (!outputOptions.dir) {
          this.error('无法确定构建产物目录');
        }
        const configPath = resolve(outputOptions.dir, 'config.js');
        const source = await readFile(configPath, 'utf8');
        const passwordLoginEnabledPattern =
          /^([ \t]*passwordLoginEnabled:[ \t]*)(?:true|false)(?=,)/gm;
        if ([...source.matchAll(passwordLoginEnabledPattern)].length !== 1) {
          this.error('构建配置必须包含唯一的 passwordLoginEnabled 布尔字段');
        }

        // 构建时写入部署地址和协议联系邮箱，单独发布的配置仍可供浏览器读取。
        let productionConfig = source.replace(
          passwordLoginEnabledPattern,
          (_match, prefix: string) => `${prefix}${String(productionPasswordLoginEnabled)}`,
        );
        const overrides = {
          apiBaseURL: process.env.API_BASE_URL?.trim() ?? '',
          contactEmail: process.env.CONTACT_EMAIL?.trim(),
        };
        for (const field of ['apiBaseURL', 'contactEmail'] as const) {
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
        const htmlPath = resolve(outputOptions.dir, 'index.html');
        const html = await readFile(htmlPath, 'utf8');
        const configReference = 'src="/config.js"';
        if (html.split(configReference).length !== 2) {
          this.error('构建页面必须包含唯一的 config.js 脚本引用');
        }

        // 先写入配置再更新页面，移除不再使用的无哈希产物。
        await writeFile(resolve(outputOptions.dir, configFilename), productionConfig);
        await writeFile(htmlPath, html.replace(configReference, `src="/${configFilename}"`));
        await rm(configPath);
      },
    },
  ],
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

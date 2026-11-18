import js from '@eslint/js';
import query from '@tanstack/eslint-plugin-query';
import prettier from 'eslint-config-prettier/flat';
import { createTypeScriptImportResolver } from 'eslint-import-resolver-typescript';
import { importX } from 'eslint-plugin-import-x';
import jsxA11y from 'eslint-plugin-jsx-a11y';
import react from 'eslint-plugin-react';
import reactHooks from 'eslint-plugin-react-hooks';
import reactRefresh from 'eslint-plugin-react-refresh';
import simpleImportSort from 'eslint-plugin-simple-import-sort';
import { defineConfig, globalIgnores } from 'eslint/config';
import tseslint from 'typescript-eslint';

export default defineConfig(
  globalIgnores([
    'dist/**',
    'coverage/**',
    'node_modules/**',
    'public/extra-assets/js/tencent/TJCaptcha.js',
  ]),
  {
    files: ['**/*.{js,mjs}'],
    extends: [js.configs.recommended],
  },
  {
    files: ['**/*.{ts,tsx}'],
    extends: [
      js.configs.recommended,
      tseslint.configs.strictTypeChecked,
      tseslint.configs.stylisticTypeChecked,
      importX.flatConfigs.recommended,
      importX.flatConfigs.typescript,
      ...query.configs['flat/recommended'],
    ],
    languageOptions: {
      parserOptions: { projectService: true, tsconfigRootDir: import.meta.dirname },
    },
    settings: {
      'import-x/resolver-next': [createTypeScriptImportResolver({ project: './tsconfig.json' })],
    },
    plugins: { 'simple-import-sort': simpleImportSort },
    rules: {
      '@typescript-eslint/consistent-type-imports': ['error', { fixStyle: 'inline-type-imports' }],
      '@typescript-eslint/consistent-type-exports': 'error',
      '@typescript-eslint/no-import-type-side-effects': 'error',
      '@typescript-eslint/no-non-null-assertion': 'error',
      '@typescript-eslint/switch-exhaustiveness-check': 'error',
      '@typescript-eslint/no-unnecessary-condition': 'error',
      'simple-import-sort/imports': 'error',
      'simple-import-sort/exports': 'error',
      'import-x/no-cycle': ['error', { ignoreExternal: true }],
      'import-x/no-duplicates': 'error',
      'import-x/no-self-import': 'error',
      'import-x/no-unresolved': 'error',
      'no-console': ['error', { allow: ['warn', 'error'] }],
      'no-alert': 'error',
      eqeqeq: ['error', 'always'],
      curly: ['error', 'all'],
      'no-var': 'error',
      'prefer-const': 'error',
    },
  },
  {
    files: ['src/**/*.tsx'],
    extends: [
      react.configs.flat.recommended,
      react.configs.flat['jsx-runtime'],
      reactHooks.configs.flat['recommended-latest'],
      jsxA11y.flatConfigs.strict,
      reactRefresh.configs.vite,
    ],
    settings: { react: { version: 'detect' } },
    rules: {
      'react/prop-types': 'off',
      'react/self-closing-comp': 'error',
      'react/jsx-no-useless-fragment': 'error',
      'react/jsx-curly-brace-presence': ['error', { props: 'never', children: 'never' }],
      'jsx-a11y/label-has-associated-control': ['error', { controlComponents: ['Checkbox'] }],
      // 本项目未启用 React Compiler，相关库按普通 React 模式使用。
      'react-hooks/incompatible-library': 'off',
      'react-refresh/only-export-components': [
        'error',
        { allowConstantExport: true, allowExportNames: ['buttonVariants', 'badgeVariants'] },
      ],
    },
  },
  {
    files: ['src/**/*.tsx'],
    ignores: ['src/components/ui/**', 'src/**/*.test.tsx', 'src/test/**'],
    rules: {
      'no-restricted-syntax': [
        'error',
        {
          selector:
            'JSXOpeningElement[name.name=/^(button|input|textarea|select|datalist|details|summary|dialog)$/]',
          message: '请使用项目公共组件，禁止直接使用 HTML 原生交互控件。',
        },
        {
          selector:
            'JSXOpeningElement[name.name="Input"] > JSXAttribute[name.name="type"][value.value=/^(date|month|time|datetime-local|week|file|checkbox|radio|color|range)$/]',
          message: '请使用对应的自定义选择组件，禁止使用浏览器原生选择器。',
        },
      ],
    },
  },
  {
    files: ['src/**/*.test.tsx', 'src/test/**'],
    rules: { 'react-refresh/only-export-components': 'off' },
  },
  prettier,
);

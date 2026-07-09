import js from "@eslint/js";
import tseslint from "typescript-eslint";

export default tseslint.config(
  // 基础推荐规则
  js.configs.recommended,

  // TypeScript 推荐规则
  ...tseslint.configs.recommended,

  {
    rules: {
      // Hardhat 测试中允许使用 any 和 require
      "@typescript-eslint/no-explicit-any": "off",
      "@typescript-eslint/no-require-imports": "off",

      // 未使用变量的处理
      "@typescript-eslint/no-unused-vars": [
        "warn",
        { argsIgnorePattern: "^_" },
      ],

      // Chai 断言如 expect(x).to.be.true 是表达式而非语句
      "@typescript-eslint/no-unused-expressions": "off",
    },
  },

  {
    // 忽略编译产物和依赖
    ignores: [
      "node_modules/**",
      "artifacts/**",
      "cache/**",
      "dist/**",
      "types/**",
    ],
  },
);

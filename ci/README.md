# GitLab CI/CD

默认分支提交会由 5-140 的 Group Runner 自动执行：

1. 克隆当前模块所需的同级协议与内核仓库。
2. 使用 BuildKit 构建镜像，并以 `提交 SHA-Pipeline ID` 作为不可变标签推送到内网 Registry。
3. 使用跨仓库文件锁串行部署到测试环境。
4. 对业务模块注册不可变 Release 并激活，再执行 readiness 检查。
5. 失败时回滚应用镜像和活动模块版本；数据库迁移保持只前进，不执行逆向迁移。

部署状态与历史 Release 位于 `/opt/liveshop/deploy/liveshop-identity`。


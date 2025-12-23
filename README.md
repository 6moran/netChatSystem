NetChatRoom - 基于Go的网络聊天室项目
📋 项目简介
NetChatRoom 是一个基于 Go 语言开发的网络聊天室系统，采用客户端-服务器架构，支持多用户在线聊天、私聊、用户管理等功能。项目使用 Docker 进行容器化部署，便于快速搭建和运行。

✨ 主要特性
✅ 用户认证系统：注册、登录、密码验证

✅ 实时聊天：支持公共聊天室和私聊功能

✅ 在线用户管理：查看在线用户列表

✅ 心跳检测：保持连接活跃，自动处理断线

✅ 容器化部署：使用 Docker Compose 一键部署

✅ MySQL 数据库：用户信息持久化存储

✅ TCP 长连接：高效稳定的网络通信

注意事项
1. 用户名在系统中唯一
2. 同一用户名不能重复登录
3. 私聊不能给自己发送消息
4. 连接超时会被自动踢出

项目状态
项目已完成基础功能开发，支持基本的聊天室功能，可以用于学习网络编程和 Go 语言开发。

🏗️ 系统架构
text
NetChatRoom
├── 客户端 (Client)
│   ├── 用户界面交互
│   ├── 消息发送/接收
│   └── 心跳检测
├── 服务器 (Server)
│   ├── 连接管理
│   ├── 消息广播
│   ├── 用户认证
│   └── 数据库操作
└── 基础设施
    ├── MySQL 数据库
    └── Docker 容器
📁 项目结构
text
netchatroom/
├── netchat/                      # 核心业务代码
│   ├── Client/                   # 客户端代码
│   │   ├── handClient/           # 客户端处理逻辑
│   │   │   └── handClient.go     # 客户端主逻辑
│   │   └── main.go               # 客户端入口
│   ├── Server/                   # 服务器端代码
│   │   ├── handServer/           # 服务器处理逻辑
│   │   │   └── handServer.go     # 服务器主逻辑
│   │   └── main.go               # 服务器入口
│   ├── common/                   # 公共定义
│   │   └── common.go             # 公共结构体
│   ├── message/                  # 消息处理
│   │   └── message.go            # 消息编解码
│   ├── utils/                    # 工具函数
│   │   └── util.go               # 数据读写工具
│   └── db/                       # 数据库操作
│       └── db.go                 # 数据库连接与操作
├── docker-compose.yml            # Docker 编排文件
├── Dockerfile-clients            # 客户端 Dockerfile
├── Dockerfile-servers            # 服务器 Dockerfile
├── mysql-init/                   # 数据库初始化
│   └── init.sql                  # 数据库初始化脚本
├── go.mod                        # Go 模块定义
└── go.sum                        # Go 依赖校验

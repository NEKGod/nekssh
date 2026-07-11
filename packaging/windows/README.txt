NekSSH for Windows x64
======================

本机使用：双击 start-nekssh.cmd
局域网使用：右键“以管理员身份运行” start-lan.cmd

浏览器地址：http://127.0.0.1:8022

关闭程序：关闭黑色命令行窗口。

安全说明：
1. start-nekssh.cmd 只允许本机访问。
2. start-lan.cmd 会监听所有网卡并开放 Windows 防火墙 8022 端口。
3. SSH 主机指纹文件保存在 data\known_hosts。
4. 密码和私钥不会保存到磁盘。

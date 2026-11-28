module github.com/jingjiangze/realtime-platform-foundation

go 1.23

// 本仓只依赖标准库；适配器实现（云 SDK、IM SDK）由使用方在自己的私有仓库引入，
// 通过 go get 加到各自模块，不污染本仓依赖树。
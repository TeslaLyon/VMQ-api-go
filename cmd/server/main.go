package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"VMQ-api-go/internal/config"
	"VMQ-api-go/internal/handler"
	"VMQ-api-go/internal/middleware"
	"VMQ-api-go/internal/model"
	"VMQ-api-go/internal/repository"
	"VMQ-api-go/internal/scheduler"
	"VMQ-api-go/internal/service"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

// TODO：添加日志 手机监控端报错、失联的日志
// TODO: 添加定时任务，监控最后一次心跳时间，超过一定时间未更新则认为失联，发送通知
// TODO:仔细检查 type 1 微信；2 支付宝
// TODO：登陆成功后依旧是 login 页面，然后手动访问首页是成功的
func main() {
	// 加载配置
	if err := config.LoadConfig("."); err != nil {
		log.Fatalf("❌ 配置初始化失败: %v", err)
	}
	log.Println("✅ 配置文件加载成功!")

	// 设置Gin模式
	gin.SetMode(config.AppConfig.Server.Mode)

	// 1.2 连接 PostgreSQL 18 数据库
	db, err := model.InitDB()
	if err != nil {
		log.Fatalf("❌ 数据库初始化失败: %v", err)
	}

	// 初始化 Valkey 9
	if err := model.InitValkey(); err != nil {
		log.Fatalf("❌ Valkey 启动失败: %v", err)
	}

	// 初始化 Gin Web 服务

	userRepo := repository.NewUserRepository(db)
	orderRepo := repository.NewOrderRepository(db)
	qrcodeRepo := repository.NewQrcodeRepository(db)

	monitorAndroidService := service.NewMonitorAndroidService(userRepo, orderRepo)
	monitorAndroidHandler := handler.NewMonitorAndroidHandler(monitorAndroidService)

	userService := service.NewUserService(userRepo)
	qrcodeService := service.NewQrcodeService(qrcodeRepo)
	qrcodeHandler := handler.NewQrcodeHandler(qrcodeService)

	orderService := service.NewOrderService(orderRepo, userRepo)
	orderHandler := handler.NewOrderHandler(orderService, monitorAndroidService, userService)

	// 初始化定时任务调度器
	taskScheduler := scheduler.NewScheduler(orderService, monitorAndroidService)
	taskScheduler.Start()

	// 注册路由
	router := setupRoutes(monitorAndroidHandler, qrcodeHandler, orderHandler)

	// 7. 优雅启动与关闭服务
	srv := &http.Server{
		Addr:         ":" + config.AppConfig.Server.Port,
		Handler:      router,
		ReadTimeout:  config.AppConfig.Server.ReadTimeout,
		WriteTimeout: config.AppConfig.Server.WriteTimeout,
	}

	go func() {
		log.Printf("🚀 服务已启动，监听端口: %s", config.AppConfig.Server.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("❌ 监听失败: %s\n", err)
		}
	}()

	// 等待中断信号以优雅地关闭服务器
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("🔌 正在关闭服务器...")

	// 停止定时任务调度器
	taskScheduler.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal("❌ 服务器强制关闭: ", err)
	}

	log.Println("👋 服务已退出")
}

// setupRoutes 分离路由注册逻辑
func setupRoutes(monitorAndroidHandler *handler.MonitorAndroidHandler, qrcodeHandler *handler.QrcodeHandler, orderHandler *handler.OrderHandler) *gin.Engine {
	router := gin.Default()
	router.Use(cors.Default())

	router.GET("/appHeart", monitorAndroidHandler.MonitorHeart)
	router.Any("/appHeart", monitorAndroidHandler.MonitorHeart)

	openAPI := router.Group("/openapi")
	openAPI.Use(middleware.OpenAPIAuthMiddleware())
	{
		openAPI.GET("/orders", orderHandler.GetOrders)
		openAPI.POST("/orders", orderHandler.CreateOrder)
	}

	reactApi := router.Group("/payapi")
	reactApi.Use(middleware.SignAuthMiddleware())
	{
		reactApi.GET("/:order_id", orderHandler.GetOrderUnified)              //统一的订单查询
		reactApi.GET("/:order_id/status", orderHandler.GetOrderStatusUnified) // 统一的订单状态查询
	}

	// 基础路由组
	api := router.Group("/api")
	{
		// 公开接口：登录 (加上 Valkey 限流中间件)
		api.POST("/login/account", middleware.LoginRateLimit(), handler.Login)
		api.POST("/auth/refresh", handler.RefreshToken) // 注册刷新接口

		// 需要 JWT 鉴权的接口
		auth := api.Group("").Use(middleware.JWTAuth())
		{
			// 获取当前用户信息 (Ant Design Pro 自动请求)
			auth.GET("/currentUser", handler.CurrentUser)

			auth.GET("/sysConfig", handler.SysConfig)
			auth.POST("/updateSysConfig", handler.UpdateSystemConfig)

			auth.GET("/monitorSettings", handler.MonitorSettings)

			auth.GET("/qrcodes", qrcodeHandler.GetQrcodes)
			auth.POST("/qrcodes", qrcodeHandler.CreateQrcode)
			auth.DELETE("/qrcodes/:id", qrcodeHandler.DeleteQrcode)
			auth.POST("/qrcodes/parse", qrcodeHandler.ParseQrcode)
			auth.PUT("/qrcodes/:id/status", qrcodeHandler.UpdateQrcodeStatus)
			// auth.GET("/qrcodes/generate", qrcodeHandler.GenerateQrcode) // 注释掉生成二维码的路由

			// 3. 注册路由 (这里可以加上你的 JWT Token 中间件保护)
			// r.POST("/api/upload", middleware.JWTAuth(), uploadHandler.HandleImageUpload)
			// r.POST("/api/upload", uploadHandler.HandleImageUpload)

			// 退出登录 (加入 Valkey 黑名单)
			auth.POST("/login/outLogin", handler.Logout)

			// 订单

			auth.POST("/close-expired", orderHandler.CloseExpiredOrders)   // 关闭过期订单
			auth.POST("/delete-expired", orderHandler.DeleteExpiredOrders) // 删除过期订单
			// 示例：用户管理相关 (你可以后续在这里扩展)
			// auth.GET("/users", handler.ListUsers)
		}
	}

	// 健康检查
	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "UP", "time": time.Now().Format(time.RFC3339)})
	})

	// router.GET("/test", func(ctx *gin.Context) {
	// 	randomPayPageSalt := utils.GenerateRandomString16Fast()
	// 	ctx.JSON(200, gin.H{"salt": randomPayPageSalt})
	// })

	return router
}

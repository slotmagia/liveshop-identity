# fulfillment data adapter

实现配送、物流、售后与投诉事实、命令账本与恢复查询的 Identity 数据适配器。本切片实现投诉工单与审核回执、售后工单与审核/收货回执、店铺配送规则和发货预设，以及发货单与轨迹回执。它只能实现对应 capability 暴露的仓储端口，不得成为跨 capability 直接写表的捷径。

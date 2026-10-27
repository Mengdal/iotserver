# 告警通知渠道

告警规则支持以下通知方式：

| 渠道 | 实现 | 配置位置 |
|---|---|---|
| 邮件 | 标准 SMTP 或 Twilio Email API，规则内选择 | 系统配置 → 通知配置 |
| 短信 | Twilio Messages API | 系统配置 → 通知配置 |
| WhatsApp | Twilio Messages API | 系统配置 → 通知配置 |
| Webhook | 自定义 HTTP POST | 告警规则内填写地址 |

企业微信、钉钉、飞书与语音告警已下线。启动时会自动从存量告警规则中清理这几种通知方式。

SMTP 和 Twilio 都是平台级配置。短信、WhatsApp 与 Twilio 邮件共用同一套 Account SID 与 Auth Token；收件邮箱、手机号和 WhatsApp 号码在每条告警规则里填写，因此不同租户可以各自配置收件人。

告警规则里只有一种「邮件」通知，添加后在卡片内用「发送渠道」下拉框选择 SMTP 或 Twilio Email。

通知的生效时间段可以留空；留空表示全天生效。

## 系统配置项

平台管理员在「系统配置 → 通知配置」中填写。

### SMTP

| 配置键 | 说明 |
|---|---|
| `notify.smtp.host` | SMTP 服务器，如 `smtp.example.com` |
| `notify.smtp.port` | 端口，默认 `587`；465 走隐式 TLS |
| `notify.smtp.username` | 登录用户名 |
| `notify.smtp.password` | 登录密码 / 应用专用密码 |
| `notify.smtp.from` | 发件人地址；留空时使用用户名 |
| `notify.smtp.tls` | 是否启用 TLS，默认开启 |

### Twilio

| 配置键 | 说明 |
|---|---|
| `notify.twilio.accountSid` | Twilio Account SID |
| `notify.twilio.authToken` | Twilio Auth Token |
| `notify.twilio.emailFrom` | 邮件发件地址，必须属于 Twilio 控制台已验证的 Sender Domain |
| `notify.twilio.smsFrom` | 短信发送号码或已注册的 Alphanumeric Sender ID |
| `notify.twilio.whatsappFrom` | WhatsApp 发送号码，如 `+14155238886` |
| `notify.whatsapp.contentSid` | WhatsApp 模板 Content SID；留空时发送纯文本 |
| `notify.phone.defaultCountryCode` | 本地号码默认国家码，沙特填 `966` |

短信、WhatsApp 与 Twilio 邮件共用同一套 Twilio 账号。Twilio 邮件通过 `https://comms.twilio.com/v1/Emails` 发送，发件域名需要先在 Twilio 控制台完成验证。WhatsApp 业务主动消息建议使用审核通过的模板；模板变量顺序约定为：

1. 设备名称
2. 告警规则名称
3. 告警级别
4. 触发时间
5. 触发条件

未配置 `ContentSid` 时回退为 `Body` 文本，仅适用于 Sandbox 或 24 小时会话窗口内。

### Twilio Trial 账号限制

Trial 账号可以验证凭据和网络，但不能直接用自定义内容发告警：

- 短信会返回 `572006 Invalid template name. Trial accounts can only use predefined SMS templates.`
- WhatsApp 会返回 `21654 ContentSid Required`，必须使用审核通过的模板
- Twilio Email 会返回 `Invalid template: email content does not match any approved template`

正式使用需要升级账号，并分别完成：Twilio 邮件 Sender Domain 验证、WhatsApp 模板审核、短信 Sender ID 注册。如果继续用 SMTP 发邮件，则不受 Twilio Trial 的邮件模板限制。


## 沙特阿拉伯

Twilio 在沙特可以用于短信与 WhatsApp，但短信有额外的准入要求：

- **必须注册 Alphanumeric Sender ID**，数字长号码和短号不可用；未注册的发送方会被沙特运营商拦截。
- 注册需要公司抬头纸、盖章的 NOC（以及 Etisalat 的 NOC），审核与开通约 **2 周**。
- Twilio 不能为**沙特本地品牌**注册 Sender ID（当地法规禁止转售本地流量）。如果客户是沙特本地企业，短信建议改用 Unifonic、Taqnyat、Infobip 等本地聚合商；Twilio 仍可用于 WhatsApp。
- 不支持双向短信；营销类 Sender ID 需要加 `-AD` 后缀，且仅允许在当地 09:00–20:00 发送。
- 内容限制：禁止赌博、成人、贷款、政治/宗教、管制物品、酒类、WhatsApp/LINE 链接、正文中的电话号码和短链接。
- WhatsApp 通过 Twilio（Meta BSP）可用；需要 Meta Business 验证，通常 3–5 个工作日，并需审核消息模板。

参考：

- [Twilio: Saudi Arabia SMS Guidelines](https://static1.twilio.com/en-us/guidelines/sa/sms)
- [Twilio: Alphanumeric Sender ID registration in Saudi Arabia](https://help.twilio.com/articles/360039841894)

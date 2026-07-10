import type { ParameterRow } from "../ParameterEditor";

export type HTTPTemplate = {
  id: string;
  title: string;
  description: string;
  commandName: string;
  commandDescription: string;
  config: Record<string, unknown>;
  params: ParameterRow[];
};

export const HTTP_TEMPLATES: HTTPTemplate[] = [
  {
    id: "chatgpt-api",
    title: "ChatGPT 手机端调用 API",
    description: "邮件正文传 message，MailRelay 拼到 Path 并调用外部接口。",
    commandName: "push",
    commandDescription: "从邮件触发一次外部 API 调用",
    config: {
      method: "GET",
      url: "https://api.example.com/push/{{message}}",
      query: { source: "chatgpt" },
    },
    params: [
      { name: "message", description: "要发送给 API 的文本", type: "string", required: true, sensitive: false, example: "hello" },
    ],
  },
  {
    id: "json-post",
    title: "JSON POST",
    description: "把邮件参数组装成 JSON 请求体，适合普通 Webhook 或 API。",
    commandName: "send-event",
    commandDescription: "发送 JSON 事件到外部 API",
    config: {
      method: "POST",
      url: "https://api.example.com/events",
      headers: { "Content-Type": "application/json" },
      body: '{\n  "message": "{{message}}"\n}',
    },
    params: [
      { name: "message", description: "事件内容", type: "string", required: true, sensitive: false, example: "hello" },
    ],
  },
  {
    id: "bark",
    title: "Bark 推送",
    description: "用 Bark API 发手机通知，Key 建议放在环境变量里。",
    commandName: "notify",
    commandDescription: "发送 Bark 手机通知",
    config: {
      method: "GET",
      url: "https://api.day.app/${BARK_KEY}/{{title}}/{{message}}",
    },
    params: [
      { name: "title", description: "通知标题", type: "string", required: true, sensitive: false, example: "MailRelay" },
      { name: "message", description: "通知内容", type: "string", required: true, sensitive: false, example: "hello" },
    ],
  },
];

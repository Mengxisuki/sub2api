import overview from './overview'
import channels from './channels'
import accounts from './accounts'
import resources from './resources'
import ops from './ops'
import settings from './settings'
import audit from './audit'
import promptAudit from './promptAudit'

export default {
  ...overview,
  ...channels,
  ...accounts,
  ...resources,
  ...ops,
  ...settings,
  ...audit,
  ...promptAudit,
  tlsFingerprintProfiles: {
    title: 'TLS 指纹模板管理',
    description: '管理用于模拟客户端 TLS 指纹的模板。',
    createProfile: '新建模板',
    editProfile: '编辑模板',
    deleteProfile: '删除模板',
    noProfiles: '暂无 TLS 指纹模板',
    createFirstProfile: '创建第一个模板开始使用。',
    loadFailed: '加载 TLS 指纹模板失败',
    createSuccess: 'TLS 指纹模板创建成功',
    updateSuccess: 'TLS 指纹模板更新成功',
    deleteSuccess: 'TLS 指纹模板删除成功',
    saveFailed: '保存 TLS 指纹模板失败',
    deleteFailed: '删除 TLS 指纹模板失败',
    deleteConfirmMessage: '确定要删除模板“{name}”吗？',
    columns: {
      name: '名称',
      description: '描述',
      grease: 'GREASE',
      alpn: 'ALPN',
      actions: '操作'
    },
    form: {
      pasteYaml: '粘贴 YAML 配置（可选）',
      pasteYamlPlaceholder: '将 tls-fingerprint-web 导出的 YAML 粘贴到这里',
      parseYaml: '解析 YAML',
      pasteYamlHint: '支持从指纹采集器复制配置。',
      openCollector: '打开采集器',
      name: '名称',
      namePlaceholder: '例如：Claude Code Bun',
      description: '描述',
      descriptionPlaceholder: '模板用途说明',
      enableGrease: '启用 GREASE',
      enableGreaseHint: '在 ClientHello 中发送 GREASE 扩展。',
      cipherSuites: '密码套件',
      cipherSuitesHint: '支持十进制或 0x 十六进制值，以逗号分隔。',
      curves: '椭圆曲线',
      curvesHint: '以逗号分隔。',
      signatureAlgorithms: '签名算法',
      supportedVersions: '支持的 TLS 版本',
      keyShareGroups: '密钥共享组',
      extensions: '扩展',
      pointFormats: '点格式',
      pskModes: 'PSK 模式',
      alpnProtocols: 'ALPN 协议',
      yamlParsed: 'YAML 解析成功',
      yamlParseFailed: '未找到有效的 YAML 模板名称'
    }
  },
}

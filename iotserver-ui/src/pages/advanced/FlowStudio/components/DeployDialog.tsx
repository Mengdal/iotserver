import React, { useEffect, useRef, useState } from 'react'
import { Alert, Button, Modal, Tag } from 'antd'
import {
  CheckCircleFilled,
  CloudUploadOutlined,
  LoadingOutlined,
  SafetyCertificateOutlined,
} from '@ant-design/icons'
import { t } from '@/i18n'
import type { CompiledArtifact, Diagnostic, FlowDoc } from '../model/types'

export interface DeployDialogProps {
  open: boolean
  flow: FlowDoc
  diagnostics: Diagnostic[]
  artifact: CompiledArtifact | null
  onClose: () => void
  onDeployed: (artifact: CompiledArtifact) => void
  onOpenRuntime: () => void
}

type Stage = 'idle' | 'running' | 'success'

const DeployDialog: React.FC<DeployDialogProps> = ({
  open,
  flow,
  diagnostics,
  artifact,
  onClose,
  onDeployed,
  onOpenRuntime,
}) => {
  const [stage, setStage] = useState<Stage>('idle')
  const [step, setStep] = useState(0)
  const timers = useRef<number[]>([])

  const errors = diagnostics.filter(item => item.severity === 'error')

  useEffect(() => {
    if (open) {
      setStage('idle')
      setStep(0)
    }
    return () => {
      timers.current.forEach(timer => window.clearTimeout(timer))
      timers.current = []
    }
  }, [open])

  const start = () => {
    if (!artifact || errors.length) return
    setStage('running')
    setStep(0)
    const runStep = (index: number) => {
      const timer = window.setTimeout(() => {
        setStep(index + 1)
        if (index < 2) {
          runStep(index + 1)
        } else {
          setStage('success')
          onDeployed(artifact)
        }
      }, 520)
      timers.current.push(timer)
    }
    runStep(0)
  }

  const steps = [
    { title: t('校验草稿'), desc: errors.length ? t('{{count}} 个错误待修复', { count: errors.length }) : t('通过') },
    { title: t('编译为图规则'), desc: artifact ? `${artifact.ruleId} · v${artifact.compilerVersion}` : t('等待编译') },
    { title: t('下发 eKuiper'), desc: stage === 'success' ? t('已下发（演示）') : t('等待下发') },
  ]

  return (
    <Modal
      open={open}
      onCancel={onClose}
      width={560}
      title={<span><CloudUploadOutlined /> {t('部署流程')}</span>}
      footer={[
        <Button key="cancel" onClick={onClose}>{stage === 'success' ? t('关闭') : t('取消')}</Button>,
        stage === 'success'
          ? <Button key="runtime" type="primary" onClick={onOpenRuntime}>{t('查看运行状态')}</Button>
          : (
            <Button
              key="deploy"
              type="primary"
              icon={stage === 'running' ? <LoadingOutlined /> : <CloudUploadOutlined />}
              disabled={!artifact || errors.length > 0 || stage === 'running'}
              loading={stage === 'running'}
              onClick={start}
            >
              {stage === 'running' ? t('部署中…') : t('开始部署')}
            </Button>
          ),
      ]}
    >
      <div className="fs-deploy">
        <Alert
          type="info"
          showIcon
          message={t('演示模式：只生成并展示编译产物，不会真正写入 eKuiper。')}
        />
        {errors.length > 0 && (
          <Alert
            type="error"
            showIcon
            message={t('校验未通过，请先修复 {{count}} 个错误。', { count: errors.length })}
          />
        )}
        <div className="fs-deploy__target">
          <div>
            <div className="fs-deploy__label">{t('部署目标')}</div>
            <div className="fs-deploy__value">{t('本机 eKuiper 2.4.2')}</div>
          </div>
          <Tag color="green">{t('在线')}</Tag>
        </div>
        <div className="fs-deploy__meta">
          <span>{t('节点数')}: {flow.spec.nodes.length}</span>
          <span>{t('语义哈希')}: {artifact ? artifact.semanticHash : '-'}</span>
        </div>

        {stage === 'success' && (
          <div className="fs-deploy__success">
            <CheckCircleFilled />
            <div>
              <div className="fs-deploy__success-title">{t('部署成功（演示）')}</div>
              <div className="fs-deploy__success-desc">
                {t('规则 {{ruleId}} 已生成，可到“运行状态”查看模拟指标。', { ruleId: artifact?.ruleId })}
              </div>
            </div>
          </div>
        )}

        <div className="fs-deploy__steps">
          {steps.map((item, index) => {
            const done = stage === 'success' || (stage === 'running' ? index < step : false)
            const active = stage === 'running' && index === step
            return (
              <div className={`fs-deploy__step${done ? ' is-done' : ''}${active ? ' is-active' : ''}`} key={item.title}>
                <span className="fs-deploy__step-icon">
                  {done ? <CheckCircleFilled /> : active ? <LoadingOutlined /> : index + 1}
                </span>
                <div>
                  <div className="fs-deploy__step-title">{item.title}</div>
                  <div className="fs-deploy__step-desc">{item.desc}</div>
                </div>
              </div>
            )
          })}
        </div>

        {!artifact && (
          <Alert type="warning" showIcon message={t('尚未生成编译产物，请先点击“编译”。')} />
        )}
        <div className="fs-deploy__note">
          <SafetyCertificateOutlined /> {t('真实部署需要后端提供编译、修订与部署记录接口。')}
        </div>
      </div>
    </Modal>
  )
}

export default DeployDialog

import React, { useCallback, useState } from 'react'
import { Button, Modal, Space, Upload, message } from 'antd'
import { DeleteOutlined, PlusOutlined } from '@ant-design/icons'
import type { RcFile } from 'antd/es/upload'

const { useMessage } = message

export interface ImageUploaderProps {
  /** 当前值：data URL 或图片地址 */
  value?: string
  onChange?: (value: string) => void
  /** 预览框宽高 */
  width?: number
  height?: number
  disabled?: boolean
}

/** 读取文件为 data URL */
function readAsDataURL (file: File): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader()
    reader.onload = () => resolve(String(reader.result || ''))
    reader.onerror = reject
    reader.readAsDataURL(file)
  })
}

/** 用 canvas 压缩，避免把大图塞进配置表 */
function compress (dataUrl: string, maxSize: number, quality = 0.9): Promise<string> {
  return new Promise(resolve => {
    const img = new Image()
    img.onload = () => {
      const scale = Math.min(1, maxSize / Math.max(img.width, img.height))
      const w = Math.max(1, Math.round(img.width * scale))
      const h = Math.max(1, Math.round(img.height * scale))
      const canvas = document.createElement('canvas')
      canvas.width = w
      canvas.height = h
      const ctx = canvas.getContext('2d')
      // svg 原样保存，避免位图化失真
      if (dataUrl.startsWith('data:image/svg') || !ctx) {
        resolve(dataUrl)
        return
      }
      ctx.drawImage(img, 0, 0, w, h)
      const isPng = dataUrl.startsWith('data:image/png')
      resolve(canvas.toDataURL(isPng ? 'image/png' : 'image/jpeg', quality))
    }
    img.onerror = () => resolve(dataUrl)
    img.src = dataUrl
  })
}

/**
 * 图片上传：单张。方框内预览，悬浮显示「预览 / 删除」，未选图时显示「+ 选择图片」。
 * 图片以 data URL 回填表单，随系统配置一起保存。
 */
const ImageUploader: React.FC<ImageUploaderProps> = ({ value, onChange, width = 120, height = 120, disabled }) => {
  const [messageApi, contextHolder] = useMessage()
  const [hover, setHover] = useState(false)
  const [previewOpen, setPreviewOpen] = useState(false)

  const beforeUpload = useCallback(async (file: RcFile) => {
    if (!/^image\//.test(file.type)) {
      messageApi.open({ type: 'error', content: '只能上传图片文件' })
      return Upload.LIST_IGNORE
    }
    if (file.size > 5 * 1024 * 1024) {
      messageApi.open({ type: 'error', content: '图片不能超过 5MB' })
      return Upload.LIST_IGNORE
    }
    const raw = await readAsDataURL(file)
    // 按预览框尺寸推导压缩上限：背景图这类大图放宽到 1200，小图 512
    const compressed = await compress(raw, Math.max(width, height) > 200 ? 1200 : 512)
    onChange?.(compressed)
    return Upload.LIST_IGNORE
  }, [messageApi, onChange, width, height])

  return (
    <Space align="start" size={12}>
      {contextHolder}
      <div
        style={{
          width,
          height,
          border: '1px dashed #d9d9d9',
          borderRadius: 8,
          background: '#fafafa',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          overflow: 'hidden',
          position: 'relative',
        }}
        onMouseEnter={() => setHover(true)}
        onMouseLeave={() => setHover(false)}
      >
        {value
          ? (
            <>
              <img src={value} alt="preview" style={{ maxWidth: '100%', maxHeight: '100%', objectFit: 'contain' }} />
              {!disabled && hover && (
                <div
                  style={{
                    position: 'absolute',
                    inset: 0,
                    background: 'rgba(0,0,0,0.45)',
                    color: '#fff',
                    display: 'flex',
                    alignItems: 'center',
                    justifyContent: 'center',
                    gap: 16,
                    fontSize: 12,
                  }}
                >
                  <span style={{ cursor: 'pointer' }} onClick={() => setPreviewOpen(true)}>预览</span>
                  <span style={{ cursor: 'pointer' }} onClick={() => onChange?.('')}>删除</span>
                </div>
              )}
            </>
          )
          : (
            <Upload accept="image/*" showUploadList={false} beforeUpload={beforeUpload} disabled={disabled}>
              <div style={{ textAlign: 'center', color: '#8c8c8c', cursor: disabled ? 'not-allowed' : 'pointer' }}>
                <PlusOutlined style={{ fontSize: 18 }} />
                <div style={{ fontSize: 12, marginTop: 4 }}>选择图片</div>
              </div>
            </Upload>
          )}
      </div>

      {value && !disabled && (
        <Space direction="vertical" size={4}>
          <Upload accept="image/*" showUploadList={false} beforeUpload={beforeUpload}>
            <Button size="small">替换</Button>
          </Upload>
          <Button size="small" danger icon={<DeleteOutlined />} onClick={() => onChange?.('')}>删除</Button>
        </Space>
      )}

      <Modal open={previewOpen} footer={null} onCancel={() => setPreviewOpen(false)} title="预览">
        <img src={value} alt="preview" style={{ width: '100%' }} />
      </Modal>
    </Space>
  )
}

export default ImageUploader

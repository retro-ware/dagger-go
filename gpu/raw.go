package gpu

import "github.com/gogpu/wgpu"

// Raw access for renderers of their own (dagger-go's Hardware 3D): the device, the queue,
// the frame's command encoder and an image's texture. Like every other call of this
// package they are for the main goroutine.

// Device is the WebGPU device (made on first use).
func Device() *wgpu.Device {
	needDevice()
	return dev.device
}

// Queue is the device's queue.
func Queue() *wgpu.Queue {
	needDevice()
	return dev.queue
}

// Encoder is the frame's command encoder with the package's own drawing flushed and its
// pass ended, so a caller can record passes and copies of its own in order.
func Encoder() *wgpu.CommandEncoder {
	needDevice()
	flushBatch()
	endPass()
	return encoder()
}

// Submit sends everything recorded so far to the GPU.
func Submit() { submit() }

// Discard drops everything recorded since the last submit, as after an encoder error (an
// encoder in its error state cannot finish, and would take the next frames with it).
func Discard() {
	flushBatch()
	if dev.pass != nil {
		_ = dev.pass.End()
		dev.pass, dev.on = nil, nil
	}
	dev.enc = nil
	dev.uniforms.reset()
	dev.vertices.reset()
}

// Texture is the image's texture and its view (the whole texture, not just the image's
// rectangle).
func (i *Image) Texture() (*wgpu.Texture, *wgpu.TextureView) { return i.t.tex, i.t.view }

module github.com/memmaker/dagger-go

go 1.26.2

require gpu v0.0.0

replace gpu => ../por-go/gpu

replace github.com/gogpu/wgpu => ../gogpu-wgpu

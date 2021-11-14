module lxsoft/amwj/server

go 1.17

replace lxsoft/amwj/core => ../core

replace lxsoft/amwj/filter => ../filter

replace lxsoft/amwj/api => ../api

replace lxsoft/amwj/logx => ../logx

replace lxsoft/amwj/data => ../data

replace lxsoft/amwj/geo => ../geo

require lxsoft/amwj/filter v0.0.0-00010101000000-000000000000

require (
	lxsoft/amwj/api v0.0.0-00010101000000-000000000000
	lxsoft/amwj/core v0.0.0-00010101000000-000000000000
	lxsoft/amwj/data v0.0.0-00010101000000-000000000000
	lxsoft/amwj/logx v0.0.0-00010101000000-000000000000
)

require lxsoft/amwj/geo v0.0.0-00010101000000-000000000000 // indirect

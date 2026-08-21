// Package dogfood materializes a realistic NestJS microservice fixture and
// scores the api-doc gates against a seeded ground truth. It is the repeatable
// dogfooding standard for the API documentation checks: every blocking omission
// in the fixture (decorator-level and business-logic error contracts) must be
// surfaced by either the static gate or the review evidence contract.
package dogfood

import (
	"fmt"
	"os"
	"path/filepath"
)

// File is one fixture file written into the materialized repo.
type File struct {
	Path    string
	Content string
}

// ExpectedFinding is one seeded blocking omission the gates must surface.
type ExpectedFinding struct {
	ID      string
	File    string
	Layer   string // "static" or "review"
	Code    string // static violation code, empty for review-layer findings
	Details []string
}

const UsersController = `import { Body, Controller, Delete, Get, Param, Patch, Post, Query, Req, UseGuards } from '@nestjs/common';
import { ApiBearerAuth, ApiBadRequestResponse, ApiBody, ApiOperation, ApiParam, ApiQuery } from '@nestjs/swagger';
import { JwtAuthGuard } from '../auth/jwt-auth.guard';
import { CreateUserDto } from './dto/create-user.dto';
import { SearchUsersDto } from './dto/search-users.dto';
import { UpdateUserDto } from './dto/update-user.dto';
import { UsersService } from './users.service';

@ApiBearerAuth()
@Controller('users')
export class UsersController {
  constructor(private readonly usersService: UsersService) {}

  @Post()
  @ApiOperation({
    summary: 'Register a new user',
    description: '### 목적\n- 신규 사용자를 등록한다.\n\n### 요청 규칙\n- 이메일은 고유해야 한다.',
  })
  @ApiUnauthorizedResponse({ description: '인증 필요' })
  @ApiBody({ type: CreateUserDto })
  @ApiBadRequestResponse({ description: '요청 본문 검증 실패' })
  create(@Body() dto: CreateUserDto) {
    return this.usersService.create(dto);
  }

  @Get(':id')
  @ApiOperation({
    summary: 'Get a user by id',
    description: '### 목적\n- 사용자 한 명을 조회한다.',
  })
  @ApiParam({ name: 'id', example: '42' })
  findOne(@Param('id') id: string) {
    return this.usersService.findOne(id);
  }

  @Get()
  @ApiOperation({
    summary: 'Search users',
    description: '### 목적\n- 사용자를 검색한다.',
  })
  @ApiQuery({ name: 'keyword', required: true })
  search(@Query() query: SearchUsersDto) {
    return this.usersService.search(query.keyword);
  }

  @Get(':id/profile')
  @ApiOperation({
    summary: 'Read the private profile',
    description: '### 목적\n- 비공개 프로필을 소유자가 조회한다.',
  })
  @ApiParam({ name: 'id' })
  getProfile(@Param('id') id: string, @Req() req: any) {
    return this.usersService.getProfile(id, req.user.id);
  }

  @Delete(':id')
  @UseGuards(JwtAuthGuard)
  remove(@Param('id') id: string) {
    return this.usersService.remove(id);
  }

  @Patch(':id')
  @ApiOperation({
    summary: 'Update a user',
    description: '### 목적\n- 사용자 정보를 수정한다.',
  })
  @ApiParam({ name: 'id' })
  @ApiBody({ type: UpdateUserDto })
  @ApiBadRequestResponse({ description: '요청 본문 검증 실패' })
  update(@Param('id') id: string, @Body() dto: UpdateUserDto) {
    return this.usersService.update(id, dto);
  }
}
`

const UsersService = `import { ConflictException, ForbiddenException, Injectable, NotFoundException } from '@nestjs/common';
import { CreateUserDto } from './dto/create-user.dto';
import { UpdateUserDto } from './dto/update-user.dto';

@Injectable()
export class UsersService {
  constructor(private readonly repo: UserRepository) {}

  async create(dto: CreateUserDto) {
    const existing = await this.repo.findByEmail(dto.email);
    if (existing) {
      throw new ConflictException('email already registered');
    }
    return this.repo.save(dto);
  }

  async findOne(id: string) {
    const user = await this.repo.findById(id);
    if (!user) {
      throw new NotFoundException('user ' + id + ' not found');
    }
    return user;
  }

  async search(keyword: string) {
    return this.repo.search(keyword);
  }

  async getProfile(id: string, requesterId: string) {
    const user = await this.findOne(id);
    if (user.id !== requesterId) {
      throw new ForbiddenException('only the owner can read the private profile');
    }
    return user;
  }

  async remove(id: string) {
    const user = await this.findOne(id);
    await this.repo.delete(user.id);
  }

  async update(id: string, dto: UpdateUserDto) {
    const user = await this.findOne(id);
    if (dto.email && dto.email !== user.email) {
      const existing = await this.repo.findByEmail(dto.email);
      if (existing) {
        throw new ConflictException('email already registered');
      }
    }
    return this.repo.save({ ...user, ...dto });
  }
}
`

const CreateUserDto = `import { ApiProperty, ApiPropertyOptional, IsNotEmpty, IsOptional, IsString } from '...';

export class CreateUserDto {
  @ApiProperty({ description: '표시 이름' })
  @IsString()
  name: string;

  @IsEmail()
  email: string;

  @ApiPropertyOptional({ description: '닉네임' })
  nickname?: string;

  @IsOptional()
  @IsString()
  marketingOptIn?: string;

  @ApiProperty({ required: false, description: '전화번호' })
  @IsString()
  phone: string;

  @IsOptional()
  @ApiProperty({ required: true, description: '선호 로케일' })
  locale?: string;

  @ApiProperty({ description: '가입 채널' })
  @IsNotEmpty()
  channel: string;
}
`

const UpdateUserDto = `import { ApiProperty, ApiPropertyOptional, IsOptional, IsString } from '...';

export class UpdateUserDto {
  @ApiPropertyOptional({ description: '표시 이름' })
  @IsOptional()
  @IsString()
  name?: string;

  @ApiPropertyOptional({ description: '닉네임' })
  @IsOptional()
  nickname?: string;
}
`

const SearchUsersDto = `import { ApiProperty } from '...';

export class SearchUsersDto {
  @ApiProperty({ description: '검색 키워드', example: 'dev' })
  keyword: string;
}
`

const GatewayOrdersController = `import { Body, Controller, Get, Inject, Param, Post } from '@nestjs/common';
import { ClientProxy } from '@nestjs/microservices';
import { ApiBadRequestResponse, ApiBearerAuth, ApiBody, ApiOperation, ApiParam } from '@nestjs/swagger';
import { CreateOrderDto } from './dto/create-order.dto';

@ApiBearerAuth()
@Controller('orders')
export class OrdersController {
  constructor(@Inject('ORDERS_CLIENT') private readonly client: ClientProxy) {}

  @Get(':id')
  @ApiOperation({
    summary: 'Get an order',
    description: '### 목적\n- 주문 한 건을 조회한다.',
  })
  @ApiParam({ name: 'id' })
  findOne(@Param('id') id: string) {
    return this.client.send({ cmd: 'orders_find' }, { id }).toPromise();
  }

  @Post()
  @ApiOperation({
    summary: 'Create an order',
    description: '### 목적\n- 주문을 생성한다.',
  })
  @ApiBody({ type: CreateOrderDto })
  @ApiBadRequestResponse({ description: '요청 본문 검증 실패' })
  create(@Body() dto: CreateOrderDto) {
    return this.client.send({ cmd: 'orders_create' }, dto).toPromise();
  }
}
`

const CreateOrderDto = `import { ApiProperty } from '...';

export class CreateOrderDto {
  @ApiProperty({ description: '주문 상품 ID' })
  productId: string;

  @ApiProperty({ description: '수량', example: 1 })
  quantity: number;
}
`

const RpcExceptionFilter = `import { ArgumentsHost, Catch, ExceptionFilter, HttpStatus } from '@nestjs/common';
import { RpcException } from '@nestjs/microservices';

@Catch(RpcException)
export class RpcExceptionFilter implements ExceptionFilter {
  catch(exception: RpcException, host: ArgumentsHost) {
    const response = host.switchToHttp().getResponse();
    const error = exception.getError() as { status?: string; message?: string };
    if (error?.status === 'CONFLICT') {
      response.status(HttpStatus.CONFLICT).json({ message: error.message });
      return;
    }
    if (error?.status === 'NOT_FOUND') {
      response.status(HttpStatus.NOT_FOUND).json({ message: error.message });
      return;
    }
    response.status(HttpStatus.BAD_GATEWAY).json({ message: error.message ?? 'downstream failure' });
  }
}
`

const OrdersServiceController = `import { Controller } from '@nestjs/common';
import { EventPattern, MessagePattern, Payload } from '@nestjs/microservices';
import { CreateOrderDto } from './dto/create-order.dto';
import { OrdersService } from './orders.service';

@Controller()
export class OrdersController {
  constructor(private readonly ordersService: OrdersService) {}

  @MessagePattern({ cmd: 'orders_find' })
  findOne(@Payload() payload: { id: string }) {
    return this.ordersService.findOne(payload.id);
  }

  @MessagePattern({ cmd: 'orders_create' })
  create(@Payload() dto: CreateOrderDto) {
    return this.ordersService.create(dto);
  }

  @EventPattern('orders_events')
  handleOrderEvents(@Payload() event: any) {
    this.ordersService.recordEvent(event);
  }
}
`

const OrdersService = `import { Injectable, NotFoundException } from '@nestjs/common';
import { RpcException } from '@nestjs/microservices';
import { CreateOrderDto } from './dto/create-order.dto';

@Injectable()
export class OrdersService {
  constructor(private readonly repo: OrderRepository) {}

  async findOne(id: string) {
    const order = await this.repo.findById(id);
    if (!order) {
      throw new NotFoundException('order ' + id + ' not found');
    }
    return order;
  }

  async create(dto: CreateOrderDto) {
    const product = await this.repo.findProduct(dto.productId);
    if (!product) {
      throw new RpcException({ status: 'NOT_FOUND', message: 'product not found' });
    }
    if (await this.repo.hasOpenOrder(dto.productId)) {
      throw new RpcException({ status: 'CONFLICT', message: 'open order already exists for product' });
    }
    return this.repo.save(dto);
  }

  recordEvent(event: any) {
    this.repo.append(event);
  }
}
`

// DirtyFiles are the fixture files that carry the seeded omissions.
func DirtyFiles() []File {
	return []File{
		{Path: "apps/api-gateway/src/users/users.controller.ts", Content: UsersController},
		{Path: "apps/api-gateway/src/users/users.service.ts", Content: UsersService},
		{Path: "apps/api-gateway/src/users/dto/create-user.dto.ts", Content: CreateUserDto},
		{Path: "apps/api-gateway/src/users/dto/update-user.dto.ts", Content: UpdateUserDto},
		{Path: "apps/api-gateway/src/users/dto/search-users.dto.ts", Content: SearchUsersDto},
		{Path: "apps/api-gateway/src/orders/orders.controller.ts", Content: GatewayOrdersController},
		{Path: "apps/api-gateway/src/orders/dto/create-order.dto.ts", Content: CreateOrderDto},
		{Path: "apps/api-gateway/src/common/rpc-exception.filter.ts", Content: RpcExceptionFilter},
		{Path: "apps/orders-service/src/orders.controller.ts", Content: OrdersServiceController},
		{Path: "apps/orders-service/src/orders.service.ts", Content: OrdersService},
	}
}

// CleanFiles is the same repo shape with every seeded omission fixed. The gates
// must produce zero static violations and no blocking review evidence against it.
func CleanFiles() []File {
	cleanUsersController := `import { Body, Controller, Delete, Get, Param, Patch, Post, Query, Req, UseGuards } from '@nestjs/common';
import { ApiBearerAuth, ApiBadRequestResponse, ApiBody, ApiConflictResponse, ApiForbiddenResponse, ApiNotFoundResponse, ApiOkResponse, ApiOperation, ApiParam, ApiQuery, ApiUnauthorizedResponse } from '@nestjs/swagger';
import { JwtAuthGuard } from '../auth/jwt-auth.guard';
import { CreateUserDto } from './dto/create-user.dto';
import { SearchUsersDto } from './dto/search-users.dto';
import { UpdateUserDto } from './dto/update-user.dto';
import { UsersService } from './users.service';

@ApiBearerAuth()
@Controller('users')
export class UsersController {
  constructor(private readonly usersService: UsersService) {}

  @Post()
  @ApiOperation({
    summary: 'Register a new user',
    description: '### 목적\n- 신규 사용자를 등록한다.\n\n### 요청 규칙\n- 이메일은 고유해야 한다.',
  })
  @ApiUnauthorizedResponse({ description: '인증 필요' })
  @ApiBody({ type: CreateUserDto })
  @ApiBadRequestResponse({ description: '요청 본문 검증 실패' })
  @ApiConflictResponse({ description: '이미 등록된 이메일' })
  create(@Body() dto: CreateUserDto) {
    return this.usersService.create(dto);
  }

  @Get(':id')
  @ApiOperation({
    summary: 'Get a user by id',
    description: '### 목적\n- 사용자 한 명을 조회한다.',
  })
  @ApiParam({ name: 'id', example: '42' })
  @ApiUnauthorizedResponse({ description: '인증 필요' })
  @ApiNotFoundResponse({ description: '사용자를 찾을 수 없음' })
  findOne(@Param('id') id: string) {
    return this.usersService.findOne(id);
  }

  @Get()
  @ApiOperation({
    summary: 'Search users',
    description: '### 목적\n- 사용자를 검색한다.',
  })
  @ApiQuery({ name: 'keyword', required: true })
  @ApiUnauthorizedResponse({ description: '인증 필요' })
  @ApiBadRequestResponse({ description: '쿼리 검증 실패' })
  search(@Query() query: SearchUsersDto) {
    return this.usersService.search(query.keyword);
  }

  @Get(':id/profile')
  @ApiOperation({
    summary: 'Read the private profile',
    description: '### 목적\n- 비공개 프로필을 소유자가 조회한다.',
  })
  @ApiParam({ name: 'id' })
  @ApiUnauthorizedResponse({ description: '인증 필요' })
  @ApiNotFoundResponse({ description: '사용자를 찾을 수 없음' })
  @ApiForbiddenResponse({ description: '소유자만 조회 가능' })
  getProfile(@Param('id') id: string, @Req() req: any) {
    return this.usersService.getProfile(id, req.user.id);
  }

  @Delete(':id')
  @ApiOperation({
    summary: 'Delete a user',
    description: '### 목적\n- 사용자를 삭제한다.',
  })
  @ApiParam({ name: 'id' })
  @ApiUnauthorizedResponse({ description: '인증 필요' })
  @ApiNotFoundResponse({ description: '사용자를 찾을 수 없음' })
  @UseGuards(JwtAuthGuard)
  remove(@Param('id') id: string) {
    return this.usersService.remove(id);
  }

  @Patch(':id')
  @ApiOperation({
    summary: 'Update a user',
    description: '### 목적\n- 사용자 정보를 수정한다.',
  })
  @ApiParam({ name: 'id' })
  @ApiBody({ type: UpdateUserDto })
  @ApiBadRequestResponse({ description: '요청 본문 검증 실패' })
  @ApiUnauthorizedResponse({ description: '인증 필요' })
  @ApiNotFoundResponse({ description: '사용자를 찾을 수 없음' })
  @ApiConflictResponse({ description: '이미 등록된 이메일' })
  update(@Param('id') id: string, @Body() dto: UpdateUserDto) {
    return this.usersService.update(id, dto);
  }
}
`
	cleanCreateUserDto := `import { ApiProperty, ApiPropertyOptional, IsEmail, IsNotEmpty, IsOptional, IsString } from '...';

export class CreateUserDto {
  @ApiProperty({ description: '표시 이름' })
  @IsString()
  name: string;

  @ApiProperty({ description: '이메일' })
  @IsEmail()
  email: string;

  @ApiPropertyOptional({ description: '닉네임' })
  @IsOptional()
  nickname?: string;

  @ApiPropertyOptional({ description: '마케팅 수신 동의' })
  @IsOptional()
  @IsString()
  marketingOptIn?: string;

  @ApiProperty({ description: '전화번호' })
  @IsString()
  phone: string;

  @ApiPropertyOptional({ description: '선호 로케일' })
  @IsOptional()
  locale?: string;

  @ApiProperty({ description: '가입 채널' })
  @IsNotEmpty()
  channel: string;
}
`
	cleanGatewayOrdersController := `import { Body, Controller, Get, Inject, Param, Post } from '@nestjs/common';
import { ClientProxy } from '@nestjs/microservices';
import { ApiBadRequestResponse, ApiBearerAuth, ApiBody, ApiConflictResponse, ApiNotFoundResponse, ApiOperation, ApiParam, ApiUnauthorizedResponse } from '@nestjs/swagger';
import { CreateOrderDto } from './dto/create-order.dto';

@ApiBearerAuth()
@Controller('orders')
export class OrdersController {
  constructor(@Inject('ORDERS_CLIENT') private readonly client: ClientProxy) {}

  @Get(':id')
  @ApiOperation({
    summary: 'Get an order',
    description: '### 목적\n- 주문 한 건을 조회한다.',
  })
  @ApiParam({ name: 'id' })
  @ApiUnauthorizedResponse({ description: '인증 필요' })
  @ApiNotFoundResponse({ description: '주문을 찾을 수 없음' })
  findOne(@Param('id') id: string) {
    return this.client.send({ cmd: 'orders_find' }, { id }).toPromise();
  }

  @Post()
  @ApiOperation({
    summary: 'Create an order',
    description: '### 목적\n- 주문을 생성한다.',
  })
  @ApiBody({ type: CreateOrderDto })
  @ApiBadRequestResponse({ description: '요청 본문 검증 실패' })
  @ApiUnauthorizedResponse({ description: '인증 필요' })
  @ApiNotFoundResponse({ description: '상품을 찾을 수 없음' })
  @ApiConflictResponse({ description: '이미 진행 중인 주문이 있음' })
  create(@Body() dto: CreateOrderDto) {
    return this.client.send({ cmd: 'orders_create' }, dto).toPromise();
  }
}
`
	return []File{
		{Path: "apps/api-gateway/src/users/users.controller.ts", Content: cleanUsersController},
		{Path: "apps/api-gateway/src/users/users.service.ts", Content: UsersService},
		{Path: "apps/api-gateway/src/users/dto/create-user.dto.ts", Content: cleanCreateUserDto},
		{Path: "apps/api-gateway/src/users/dto/update-user.dto.ts", Content: UpdateUserDto},
		{Path: "apps/api-gateway/src/users/dto/search-users.dto.ts", Content: SearchUsersDto},
		{Path: "apps/api-gateway/src/orders/orders.controller.ts", Content: cleanGatewayOrdersController},
		{Path: "apps/api-gateway/src/orders/dto/create-order.dto.ts", Content: CreateOrderDto},
		{Path: "apps/api-gateway/src/common/rpc-exception.filter.ts", Content: RpcExceptionFilter},
		{Path: "apps/orders-service/src/orders.controller.ts", Content: OrdersServiceController},
		{Path: "apps/orders-service/src/orders.service.ts", Content: OrdersService},
	}
}

// GroundTruth lists every seeded blocking omission with the layer that must
// surface it. Static findings are matched by violation code; review findings are
// matched by evidence details that must appear in the review input contract.
func GroundTruth() []ExpectedFinding {
	return []ExpectedFinding{
		{ID: "S1", File: "users.controller.ts", Layer: "static", Code: "missing_api_operation"},
		{ID: "S2", File: "users.controller.ts", Layer: "static", Code: "missing_api_param"},
		{ID: "S3", File: "users.controller.ts", Layer: "static", Code: "missing_400_response"},
		{ID: "S4", File: "users.controller.ts", Layer: "static", Code: "missing_401_response"},
		{ID: "S5", File: "create-user.dto.ts", Layer: "static", Code: "missing_api_property"},
		{ID: "S6", File: "create-user.dto.ts", Layer: "static", Code: "missing_api_property_optional"},
		{ID: "S7", File: "create-user.dto.ts", Layer: "static", Code: "missing_is_optional"},
		{ID: "S8", File: "create-user.dto.ts", Layer: "static", Code: "required_optional_mismatch", Details: []string{"phone"}},
		{ID: "S9", File: "create-user.dto.ts", Layer: "static", Code: "required_optional_mismatch", Details: []string{"locale"}},
		{ID: "E1", File: "users.controller.ts", Layer: "review", Details: []string{"users.service.ts", "NotFoundException", "findOne"}},
		{ID: "E2", File: "users.controller.ts", Layer: "review", Details: []string{"users.service.ts", "ConflictException", "create"}},
		{ID: "E3", File: "users.controller.ts", Layer: "review", Details: []string{"users.service.ts", "ForbiddenException", "getProfile"}},
		{ID: "E4", File: "users.controller.ts", Layer: "review", Details: []string{"orders.service.ts", "RpcException", "CONFLICT", "orders_create"}},
		{ID: "E5", File: "orders.controller.ts", Layer: "review", Details: []string{"orders.service.ts", "NotFoundException", "orders_find"}},
	}
}

// Materialize writes the fixture repo under dir.
func Materialize(dir string, files []File) error {
	for _, file := range files {
		path := filepath.Join(dir, file.Path)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return fmt.Errorf("mkdir %s: %w", path, err)
		}
		if err := os.WriteFile(path, []byte(file.Content), 0o644); err != nil {
			return fmt.Errorf("write %s: %w", path, err)
		}
	}
	return nil
}

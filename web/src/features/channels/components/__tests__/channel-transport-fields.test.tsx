/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { useForm } from 'react-hook-form'
import { expect, test, vi } from 'vitest'

import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
} from '@/components/ui/form'

import { FIELD_DESCRIPTIONS } from '../../constants'
import { httpShardItems } from '../../lib'
import {
  HttpProtocolSelect,
  HttpShardsSelect,
} from '../channel-transport-fields'

test('the protocol selector offers only auto and HTTP/1.1 without the batch keep option', async () => {
  const user = userEvent.setup()
  render(<HttpProtocolSelect value='auto' onValueChange={vi.fn()} />)

  await user.click(screen.getByRole('combobox'))

  expect(screen.getByRole('option', { name: 'Auto' })).toBeInTheDocument()
  expect(screen.getByRole('option', { name: 'HTTP/1.1' })).toBeInTheDocument()
  expect(
    screen.queryByRole('option', { name: 'Keep unchanged' })
  ).not.toBeInTheDocument()
})

test('the protocol selector adds a keep option and reports the chosen value when batch editing', async () => {
  const user = userEvent.setup()
  const onValueChange = vi.fn()
  render(
    <HttpProtocolSelect
      value='keep'
      onValueChange={onValueChange}
      allowUnchanged
    />
  )

  const trigger = screen.getByRole('combobox')
  expect(trigger).toHaveTextContent('Keep unchanged')

  await user.click(trigger)
  await user.click(screen.getByRole('option', { name: 'HTTP/1.1' }))

  expect(onValueChange).toHaveBeenCalledWith('http1')
})

test('the shard selector lists every shard count from 1 to 8', async () => {
  const user = userEvent.setup()
  render(<HttpShardsSelect value='1' onValueChange={vi.fn()} />)

  await user.click(screen.getByRole('combobox'))

  for (const item of httpShardItems) {
    expect(screen.getByRole('option', { name: item.label })).toBeInTheDocument()
  }
  expect(
    screen.queryByRole('option', { name: 'Keep unchanged' })
  ).not.toBeInTheDocument()
})

test('the shard selector is disabled while HTTP/1.1 is selected', () => {
  render(<HttpShardsSelect value='1' onValueChange={vi.fn()} disabled />)

  expect(screen.getByRole('combobox')).toBeDisabled()
})

test('inside a react-hook-form item the shared selector keeps the label and description wiring', () => {
  function Harness() {
    const form = useForm({ defaultValues: { http_protocol: 'auto' } })
    return (
      <Form {...form}>
        <form>
          <FormField
            control={form.control}
            name='http_protocol'
            render={({ field }) => (
              <FormItem>
                <FormLabel>HTTP Protocol</FormLabel>
                <FormControl>
                  <HttpProtocolSelect
                    value={field.value}
                    onValueChange={field.onChange}
                  />
                </FormControl>
                <FormDescription>
                  {FIELD_DESCRIPTIONS.HTTP_PROTOCOL}
                </FormDescription>
              </FormItem>
            )}
          />
        </form>
      </Form>
    )
  }
  render(<Harness />)

  const trigger = screen.getByRole('combobox', { name: 'HTTP Protocol' })
  const description = screen.getByText(FIELD_DESCRIPTIONS.HTTP_PROTOCOL)

  expect(trigger.getAttribute('aria-describedby')).toBe(description.id)
})
